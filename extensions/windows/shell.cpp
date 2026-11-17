#include <windows.h>
#include <atomic>
#include <chrono>
#include <mutex>
#include <new>
#include <sddl.h>
#include <shellapi.h>
#include <shlobj.h>
#include <string>
#include <unordered_map>
#include <vector>

// Independently implemented COM handlers; no Git process runs inside Explorer.
static const CLSID ids[] = {{0x29810e41,
                             0x632a,
                             0x4a9b,
                             {0xa1, 0x90, 0x4d, 0x65, 0x93, 0x66, 0x81, 0x01}},
                            {0x29810e41,
                             0x632a,
                             0x4a9b,
                             {0xa1, 0x90, 0x4d, 0x65, 0x93, 0x66, 0x81, 0x02}},
                            {0x29810e41,
                             0x632a,
                             0x4a9b,
                             {0xa1, 0x90, 0x4d, 0x65, 0x93, 0x66, 0x81, 0x03}},
                            {0x29810e41,
                             0x632a,
                             0x4a9b,
                             {0xa1, 0x90, 0x4d, 0x65, 0x93, 0x66, 0x81, 0x04}}};
static HMODULE moduleHandle;
static std::atomic<long> objects{0};
static std::wstring modulePath() {
  std::wstring p(32768, L'\0');
  DWORD n = GetModuleFileNameW(moduleHandle, p.data(), 32768);
  if (!n || n >= 32768)
    return L"";
  p.resize(n);
  return p;
}
static std::wstring pipeName() {
  HANDLE token = nullptr;
  if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token))
    return L"";
  DWORD n = 0;
  GetTokenInformation(token, TokenUser, nullptr, 0, &n);
  std::vector<BYTE> b(n);
  std::wstring result;
  if (GetTokenInformation(token, TokenUser, b.data(), n, &n)) {
    LPWSTR sid = nullptr;
    if (ConvertSidToStringSidW(
            reinterpret_cast<TOKEN_USER *>(b.data())->User.Sid, &sid)) {
      result = L"\\\\.\\pipe\\MyGittu-";
      result += sid;
      LocalFree(sid);
    }
  }
  CloseHandle(token);
  return result;
}
static std::string jsonPath(const std::wstring &s) {
  int n = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, s.data(),
                              static_cast<int>(s.size()), nullptr, 0, nullptr,
                              nullptr);
  if (n <= 0)
    return "";
  std::string u(n, 0);
  WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, s.data(),
                      static_cast<int>(s.size()), u.data(), n, nullptr,
                      nullptr);
  std::string out;
  for (unsigned char c : u) {
    if (c == '"' || c == '\\') {
      out += '\\';
      out += c;
    } else if (c < 32) {
      const char *hex = "0123456789abcdef";
      out += "\\u00";
      out += hex[c >> 4];
      out += hex[c & 15];
    } else
      out += c;
  }
  return out;
}
struct Handle {
  HANDLE h;
  ~Handle() {
    if (h != INVALID_HANDLE_VALUE && h)
      CloseHandle(h);
  }
};
static bool transfer(HANDLE pipe, bool writing, void *buffer, DWORD size,
                     ULONGLONG deadline) {
  BYTE *p = static_cast<BYTE *>(buffer);
  DWORD done = 0;
  Handle event{CreateEventW(nullptr, TRUE, FALSE, nullptr)};
  if (!event.h)
    return false;
  while (done < size) {
    OVERLAPPED ov{};
    ov.hEvent = event.h;
    ResetEvent(event.h);
    DWORD n = 0;
    BOOL ok = writing ? WriteFile(pipe, p + done, size - done, &n, &ov)
                      : ReadFile(pipe, p + done, size - done, &n, &ov);
    if (!ok) {
      if (GetLastError() != ERROR_IO_PENDING)
        return false;
      ULONGLONG now = GetTickCount64();
      DWORD remaining = now < deadline ? static_cast<DWORD>(deadline - now) : 0;
      if (WaitForSingleObject(event.h, remaining) != WAIT_OBJECT_0) {
        CancelIoEx(pipe, &ov);
        GetOverlappedResult(pipe, &ov, &n, TRUE);
        return false;
      }
      if (!GetOverlappedResult(pipe, &ov, &n, FALSE))
        return false;
    }
    if (n == 0)
      return false;
    done += n;
  };
  return true;
}
static std::string rpc(const char *method, const std::string &params) {
  static const std::wstring endpoint = pipeName();
  if (endpoint.empty())
    return {};
  ULONGLONG deadline = GetTickCount64() + 40;
  Handle pipe{CreateFileW(endpoint.c_str(), GENERIC_READ | GENERIC_WRITE, 0,
                          nullptr, OPEN_EXISTING, FILE_FLAG_OVERLAPPED,
                          nullptr)};
  if (pipe.h == INVALID_HANDLE_VALUE)
    return {};
  std::string req = "{\"version\":1,\"request_id\":\"shell\",\"method\":\"";
  req += method;
  req += "\",\"params\":" + params + "}";
  DWORD n = static_cast<DWORD>(req.size());
  BYTE header[4] = {static_cast<BYTE>(n >> 24), static_cast<BYTE>(n >> 16),
                    static_cast<BYTE>(n >> 8), static_cast<BYTE>(n)};
  if (!transfer(pipe.h, true, header, 4, deadline) ||
      !transfer(pipe.h, true, req.data(), n, deadline) ||
      !transfer(pipe.h, false, header, 4, deadline))
    return {};
  n = (DWORD(header[0]) << 24) | (DWORD(header[1]) << 16) |
      (DWORD(header[2]) << 8) | DWORD(header[3]);
  if (!n || n > 4096)
    return {};
  std::string reply(n, 0);
  if (!transfer(pipe.h, false, reply.data(), n, deadline))
    return {};
  if (reply.find("\"ok\":true") == std::string::npos)
    return {};
  return reply;
}
static bool english() {
  static std::atomic<bool> value{PRIMARYLANGID(GetUserDefaultUILanguage()) !=
                                 LANG_CHINESE};
  static std::atomic<ULONGLONG> updated{0};
  auto now = GetTickCount64();
  if (now - updated.load() > 2000) {
    updated.store(now);
    auto reply = rpc("settings.get", "{}");
    if (reply.find("\"language\":\"en\"") != std::string::npos)
      value.store(true);
    else if (reply.find("\"language\":\"zh-CN\"") != std::string::npos)
      value.store(false);
  }
  return value.load();
}
static int query(const std::wstring &path) {
  auto reply = rpc("overlay.lookup", "{\"path\":\"" + jsonPath(path) + "\"}");
  if (reply.find("\"state\":\"conflicted\"") != std::string::npos)
    return 3;
  if (reply.find("\"state\":\"clean\"") != std::string::npos)
    return 1;
  for (auto state : {"modified", "added", "deleted", "untracked"})
    if (reply.find(std::string("\"state\":\"") + state + "\"") !=
        std::string::npos)
      return 2;
  return 0;
}
static int cachedQuery(const std::wstring &path) {
  struct Entry {
    int state;
    ULONGLONG until;
  };
  static std::mutex mutex;
  static std::unordered_map<std::wstring, Entry> cache;
  std::unique_lock<std::mutex> lock(mutex, std::try_to_lock);
  if (!lock.owns_lock())
    return 0;
  auto i = cache.find(path);
  auto now = GetTickCount64();
  if (i != cache.end() && i->second.until > now)
    return i->second.state;
  int state = query(path);
  if (cache.size() >= 2048)
    cache.clear();
  cache[path] = {state, now + 500};
  return state;
}
// CommandLineToArgvW compatible quoting; the executable is never supplied by
// IPC.
static std::wstring quote(const std::wstring &s) {
  std::wstring q = L"\"";
  size_t slashes = 0;
  for (wchar_t c : s) {
    if (c == L'\\') {
      slashes++;
      continue;
    }
    if (c == L'"') {
      q.append(slashes * 2 + 1, L'\\');
      q += c;
    } else {
      q.append(slashes, L'\\');
      q += c;
    }
    slashes = 0;
  }
  q.append(slashes * 2, L'\\');
  return q + L"\"";
}
class Shell final : public IShellExtInit,
                    public IContextMenu,
                    public IShellIconOverlayIdentifier {
  std::atomic<ULONG> refs{1};
  int kind;
  std::wstring path;

public:
  explicit Shell(int k) : kind(k) { objects++; }
  ~Shell() { objects--; }
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **pp) override {
    if (!pp)
      return E_POINTER;
    *pp = nullptr;
    if (iid == IID_IUnknown || iid == IID_IShellExtInit)
      *pp = static_cast<IShellExtInit *>(this);
    else if (iid == IID_IContextMenu && kind == 0)
      *pp = static_cast<IContextMenu *>(this);
    else if (iid == IID_IShellIconOverlayIdentifier && kind > 0)
      *pp = static_cast<IShellIconOverlayIdentifier *>(this);
    else
      return E_NOINTERFACE;
    AddRef();
    return S_OK;
  }
  ULONG STDMETHODCALLTYPE AddRef() override { return ++refs; }
  ULONG STDMETHODCALLTYPE Release() override {
    ULONG n = --refs;
    if (!n)
      delete this;
    return n;
  }
  HRESULT STDMETHODCALLTYPE Initialize(PCIDLIST_ABSOLUTE folder,
                                       IDataObject *data, HKEY) override try {
    path.clear();
    if (data) {
      FORMATETC f{CF_HDROP, nullptr, DVASPECT_CONTENT, -1, TYMED_HGLOBAL};
      struct Medium {
        STGMEDIUM value{};
        void *locked{};
        ~Medium() {
          if (locked)
            GlobalUnlock(value.hGlobal);
          if (value.tymed)
            ReleaseStgMedium(&value);
        }
      } m;
      if (SUCCEEDED(data->GetData(&f, &m.value))) {
        m.locked = GlobalLock(m.value.hGlobal);
        auto drop = reinterpret_cast<HDROP>(m.locked);
        if (drop) {
          UINT n = DragQueryFileW(drop, 0, nullptr, 0);
          if (n) {
            std::vector<wchar_t> b(n + 1);
            DragQueryFileW(drop, 0, b.data(), n + 1);
            path = b.data();
          }
        }
      }
    } else if (folder) {
      std::vector<wchar_t> p(32768);
      if (SHGetPathFromIDListEx(folder, p.data(), 32768, GPFIDL_DEFAULT))
        path = p.data();
    }
    return path.empty() ? E_FAIL : S_OK;
  } catch (...) {
    return E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE QueryContextMenu(HMENU menu, UINT pos, UINT first,
                                             UINT last, UINT flags) override
      try {
    if ((flags & CMF_DEFAULTONLY) || path.empty() || last - first < 3)
      return MAKE_HRESULT(SEVERITY_SUCCESS, 0, 0);
    const wchar_t *en[] = {L"MyGittu · Status", L"MyGittu · Commit",
                           L"MyGittu · Log", L"MyGittu · Diff"};
    const wchar_t *zh[] = {L"MyGittu · 状态", L"MyGittu · 提交",
                           L"MyGittu · 历史", L"MyGittu · 差异"};
    auto labels = english() ? en : zh;
    for (UINT i = 0; i < 4; i++)
      InsertMenuW(menu, pos + i, MF_BYPOSITION | MF_STRING, first + i,
                  labels[i]);
    return MAKE_HRESULT(SEVERITY_SUCCESS, 0, 4);
  } catch (...) {
    return E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE InvokeCommand(CMINVOKECOMMANDINFO *info) override
      try {
    if (!info || path.empty())
      return E_INVALIDARG;
    const auto command = reinterpret_cast<UINT_PTR>(info->lpVerb);
    if (command > 3)
      return E_INVALIDARG;
    const wchar_t *actions[] = {L"status", L"commit", L"log", L"diff"};
    auto dll = modulePath();
    auto slash = dll.find_last_of(L"\\/");
    if (slash == std::wstring::npos)
      return E_FAIL;
    auto exe = dll.substr(0, slash + 1) + L"desktop.exe";
    auto cmd = quote(exe) + L" --action " + actions[command] + L" --path " +
               quote(path);
    STARTUPINFOW si{};
    si.cb = sizeof(si);
    PROCESS_INFORMATION pi{};
    if (!CreateProcessW(exe.c_str(), cmd.data(), nullptr, nullptr, FALSE, 0,
                        nullptr, nullptr, &si, &pi))
      return HRESULT_FROM_WIN32(GetLastError());
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    return S_OK;
  } catch (...) {
    return E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE GetCommandString(UINT_PTR, UINT, UINT *, LPSTR,
                                             UINT) override {
    return E_NOTIMPL;
  }
  HRESULT STDMETHODCALLTYPE IsMemberOf(LPCWSTR p, DWORD) noexcept override try {
    if (!p || kind == 0)
      return S_FALSE;
    return cachedQuery(p) == kind ? S_OK : S_FALSE;
  } catch (...) {
    return S_FALSE;
  }
  HRESULT STDMETHODCALLTYPE GetOverlayInfo(LPWSTR p, int capacity, int *index,
                                           DWORD *flags) noexcept override try {
    if (!p || !index || !flags)
      return E_POINTER;
    auto file = modulePath();
    if (file.empty() || static_cast<int>(file.size()) >= capacity)
      return E_FAIL;
    wcscpy_s(p, capacity, file.c_str());
    *index = kind - 1;
    *flags = ISIOI_ICONFILE | ISIOI_ICONINDEX;
    return S_OK;
  } catch (...) {
    return E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE GetPriority(int *p) noexcept override {
    if (!p)
      return E_POINTER;
    *p = kind == 3 ? 0 : kind == 2 ? 1 : 2;
    return S_OK;
  }
};
class Factory final : public IClassFactory {
  std::atomic<ULONG> refs{1};
  int kind;

public:
  explicit Factory(int k) : kind(k) { objects++; }
  ~Factory() { objects--; }
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **pp) override {
    if (!pp)
      return E_POINTER;
    *pp = nullptr;
    if (iid != IID_IUnknown && iid != IID_IClassFactory)
      return E_NOINTERFACE;
    *pp = this;
    AddRef();
    return S_OK;
  }
  ULONG STDMETHODCALLTYPE AddRef() override { return ++refs; }
  ULONG STDMETHODCALLTYPE Release() override {
    auto n = --refs;
    if (!n)
      delete this;
    return n;
  }
  HRESULT STDMETHODCALLTYPE CreateInstance(IUnknown *outer, REFIID iid,
                                           void **pp) override {
    if (outer)
      return CLASS_E_NOAGGREGATION;
    auto s = new (std::nothrow) Shell(kind);
    if (!s)
      return E_OUTOFMEMORY;
    auto hr = s->QueryInterface(iid, pp);
    s->Release();
    return hr;
  }
  HRESULT STDMETHODCALLTYPE LockServer(BOOL lock) override {
    if (lock)
      objects++;
    else
      objects--;
    return S_OK;
  }
};
extern "C" HRESULT __stdcall DllGetClassObject(REFCLSID clsid, REFIID iid,
                                               void **pp) {
  if (!pp)
    return E_POINTER;
  *pp = nullptr;
  for (int k = 0; k < 4; k++) {
    if (clsid == ids[k]) {
      auto f = new (std::nothrow) Factory(k);
      if (!f)
        return E_OUTOFMEMORY;
      auto hr = f->QueryInterface(iid, pp);
      f->Release();
      return hr;
    }
  }
  return CLASS_E_CLASSNOTAVAILABLE;
}
extern "C" HRESULT __stdcall DllCanUnloadNow() {
  return objects.load() == 0 ? S_OK : S_FALSE;
}
BOOL WINAPI DllMain(HINSTANCE module, DWORD reason, LPVOID) {
  if (reason == DLL_PROCESS_ATTACH) {
    moduleHandle = module;
    DisableThreadLibraryCalls(module);
  }
  return TRUE;
}
