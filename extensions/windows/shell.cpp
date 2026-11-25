#include <windows.h>
#include <algorithm>
#include <atomic>
#include <chrono>
#include <iterator>
#include <mutex>
#include <new>
#include <optional>
#include <sddl.h>
#include <shellapi.h>
#include <shlobj.h>
#include <shobjidl.h>
#include <string>
#include <unordered_map>
#include <vector>
#include "menu_icons.h"

// Independently implemented COM handlers; no Git process runs inside Explorer.
static const CLSID baseID = {0x29810e41, 0x632a, 0x4a9b,
                            {0xa1, 0x90, 0x4d, 0x65, 0x93, 0x66, 0x81, 0x01}};
static constexpr int modernKind = -1;

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
static std::optional<int> query(const std::wstring &path) {
  auto reply = rpc("overlay.lookup", "{\"path\":\"" + jsonPath(path) + "\"}");
  if (reply.empty())
    return std::nullopt;
  if (reply.find("\"overlay_state\":\"behind\"") != std::string::npos)
    return 8;
  constexpr const char *states[]{"clean", "modified", "conflicted", "added",
                                  "deleted", "untracked", "ignored"};
  for (int i = 0; i < static_cast<int>(std::size(states)); ++i)
    if (reply.find(std::string("\"state\":\"") + states[i] + "\"") !=
        std::string::npos)
      return i + 1;
  return 0;
}
static int cachedQuery(const std::wstring &path) {
  struct Entry {
    int state;
    ULONGLONG until;
    ULONGLONG staleUntil;
  };
  static std::mutex mutex;
  static std::unordered_map<std::wstring, Entry> cache;
  auto now = GetTickCount64();
  {
    std::lock_guard<std::mutex> lock(mutex);
    auto i = cache.find(path);
    if (i != cache.end() && i->second.until > now)
      return i->second.state;
  }
  auto state = query(path);
  std::lock_guard<std::mutex> lock(mutex);
  auto existing = cache.find(path);
  if (!state) {
    if (existing != cache.end() && existing->second.staleUntil > GetTickCount64()) {
      // One failure can otherwise make all three overlay handlers miss.
      existing->second.until = now + 100;
      return existing->second.state;
    }
    return 0;
  }
  if (*state != 0) {
    if (existing == cache.end() && cache.size() >= 2048)
      cache.erase(cache.begin());
    cache[path] = {*state, now + 200, now + 5000};
  } else {
    cache.erase(path);
  }
  return *state;
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
static bool probeRepository(std::wstring path) {
  auto known = rpc("repo.contains", "{\"path\":\"" + jsonPath(path) + "\"}");
  if (known.find("\"repository\":true") != std::string::npos)
    return true;
  if (path.size() < 3)
    return false;
  if (!(GetFileAttributesW(path.c_str()) & FILE_ATTRIBUTE_DIRECTORY)) {
    auto slash = path.find_last_of(L"\\/");
    if (slash == std::wstring::npos)
      return false;
    path.resize(slash);
  }
  for (int depth = 0; depth < 64 && path.size() >= 3; depth++) {
    if (GetFileAttributesW((path + L"\\.git").c_str()) !=
        INVALID_FILE_ATTRIBUTES)
      return true;
    auto slash = path.find_last_of(L"\\/");
    if (slash == std::wstring::npos || slash < 3)
      break;
    path.resize(slash);
  }
  return false;
}
static bool repositoryPath(const std::wstring &path) {
  struct Entry {
    bool repository;
    ULONGLONG until;
  };
  static std::mutex mutex;
  static std::unordered_map<std::wstring, Entry> cache;
  auto now = GetTickCount64();
  {
    std::lock_guard<std::mutex> lock(mutex);
    auto found = cache.find(path);
    if (found != cache.end() && found->second.until > now)
      return found->second.repository;
  }
  bool result = probeRepository(path);
  {
    std::lock_guard<std::mutex> lock(mutex);
    if (cache.size() >= 1024)
      cache.clear();
    cache[path] = {result, now + 2000};
  }
  return result;
}
static const wchar_t *actions[] = {L"status", L"commit", L"log",  L"diff",
                                   L"pull", L"push", L"clone", L"file-diff", L"file-log", L"file-blame", L"file-stage", L"file-unstage", L"file-restore", L"file-conflict"};
static constexpr int menuIconIDs[] = {
    IDI_MENU_CHANGES, IDI_MENU_COMMIT, IDI_MENU_HISTORY, IDI_MENU_DIFF,
    IDI_MENU_PULL, IDI_MENU_PUSH, IDI_MENU_CLONE, IDI_MENU_DIFF, IDI_MENU_HISTORY, IDI_MENU_BLAME, IDI_MENU_STAGE, IDI_MENU_UNSTAGE, IDI_MENU_RESTORE, IDI_MENU_CONFLICT};
static_assert(std::size(actions) == std::size(menuIconIDs));
static int menuIconResource(int action) {
  return action < 0 ? IDI_MENU_ROOT : menuIconIDs[action];
}
static constexpr int repositoryCommands[] = {4, 0, 1, 5, 2, 3};
static constexpr int cloneCommand = 6;
static constexpr int fileCommands[] = {7, 8, 9, 10, 11, 12, 13};
static std::vector<int> commandsFor(const std::wstring &path) {
  const auto attributes = GetFileAttributesW(path.c_str());
  if (attributes == INVALID_FILE_ATTRIBUTES)
    return {};
  const bool repo = repositoryPath(path);
  if (attributes & FILE_ATTRIBUTE_DIRECTORY)
    return repo ? std::vector<int>(std::begin(repositoryCommands), std::end(repositoryCommands))
                : std::vector<int>{cloneCommand};
  if (!repo)
    return {};
  // Explorer asks each modern menu child separately on the same apartment.
  static thread_local std::wstring cachedPath;
  static thread_local std::string cachedReply;
  static thread_local ULONGLONG expires = 0;
  const auto now = GetTickCount64();
  if (cachedPath != path || now >= expires) {
    cachedReply = rpc("menu.file", "{\"path\":\"" + jsonPath(path) + "\"}");
    cachedPath = path;
    expires = now + 250;
  }
  const auto &reply = cachedReply;
  if (reply.empty() || reply.find("\"ready\":false") != std::string::npos)
    return {7, 8, 9};
  std::vector<int> result;
  constexpr const char *keys[]{"diff", "history", "blame", "stage", "unstage",
                                "restore", "conflict"};
  for (size_t i = 0; i < std::size(keys); ++i)
    if (reply.find(std::string("\"") + keys[i] + "\":true") != std::string::npos)
      result.push_back(fileCommands[i]);
  return result;
}
static const wchar_t *commandTitle(int action) {
  const wchar_t *en[] = {L"Changes", L"Commit", L"History", L"Diff",
                         L"Pull",   L"Push",   L"Clone", L"File diff", L"File history", L"Blame", L"Stage file", L"Unstage file", L"Restore file", L"Resolve conflict"};
  const wchar_t *zh[] = {L"变更",  L"提交", L"历史", L"差异",
                         L"拉取", L"推送", L"克隆", L"文件差异", L"文件历史", L"逐行追溯", L"暂存此文件", L"取消暂存此文件", L"恢复此文件", L"解决冲突"};
  return english() ? en[action] : zh[action];
}
static HRESULT launch(const std::wstring &path, int action) {
  if (path.empty() || action < 0 || static_cast<size_t>(action) >= std::size(actions))
    return E_INVALIDARG;
  auto dll = modulePath();
  auto slash = dll.find_last_of(L"\\/");
  if (slash == std::wstring::npos)
    return E_FAIL;
  auto exe = dll.substr(0, slash + 1) + L"desktop.exe";
  auto cmd =
      quote(exe) + L" --action " + actions[action] + L" --path " + quote(path);
  STARTUPINFOW si{};
  si.cb = sizeof(si);
  PROCESS_INFORMATION pi{};
  if (!CreateProcessW(exe.c_str(), cmd.data(), nullptr, nullptr, FALSE, 0,
                      nullptr, nullptr, &si, &pi))
    return HRESULT_FROM_WIN32(GetLastError());
  CloseHandle(pi.hThread);
  CloseHandle(pi.hProcess);
  return S_OK;
}
static HBITMAP menuBitmap(int resource, int size) {
  auto icon =
      static_cast<HICON>(LoadImageW(moduleHandle, MAKEINTRESOURCEW(resource),
                                    IMAGE_ICON, size, size, LR_DEFAULTCOLOR));
  if (!icon)
    return nullptr;
  BITMAPINFO info{};
  info.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
  info.bmiHeader.biWidth = size;
  info.bmiHeader.biHeight = -size;
  info.bmiHeader.biPlanes = 1;
  info.bmiHeader.biBitCount = 32;
  info.bmiHeader.biCompression = BI_RGB;
  void *bits = nullptr;
  auto bitmap =
      CreateDIBSection(nullptr, &info, DIB_RGB_COLORS, &bits, nullptr, 0);
  if (bitmap) {
    ZeroMemory(bits, static_cast<size_t>(size) * size * 4);
    auto dc = CreateCompatibleDC(nullptr);
    if (dc) {
      auto old = SelectObject(dc, bitmap);
      DrawIconEx(dc, 0, 0, icon, size, size, 0, nullptr, DI_NORMAL);
      SelectObject(dc, old);
      DeleteDC(dc);
    } else {
      DeleteObject(bitmap);
      bitmap = nullptr;
    }
  }
  DestroyIcon(icon);
  return bitmap;
}
static int menuIconSize() {
  using WindowDpi = UINT(WINAPI *)(HWND);
  static const auto getDpi = reinterpret_cast<WindowDpi>(
      GetProcAddress(GetModuleHandleW(L"user32.dll"), "GetDpiForWindow"));
  const UINT dpi = getDpi ? getDpi(GetForegroundWindow()) : 0;
  return MulDiv(16, dpi ? dpi : USER_DEFAULT_SCREEN_DPI,
                USER_DEFAULT_SCREEN_DPI);
}
class Shell final : public IShellExtInit,
                    public IContextMenu,
                    public IShellIconOverlayIdentifier {
  std::atomic<ULONG> refs{1};
  int kind;
  std::wstring path;
  std::vector<int> commands;
  std::unordered_map<unsigned, HBITMAP> menuBitmaps;

  HBITMAP commandBitmap(int action, int size) {
    const int resource = menuIconResource(action);
    const auto key = (static_cast<unsigned>(size) << 16) | resource;
    auto [item, inserted] = menuBitmaps.try_emplace(key, nullptr);
    if (inserted)
      item->second = menuBitmap(resource, size);
    return item->second;
  }

public:
  explicit Shell(int k) : kind(k) {
    objects++;
  }
  ~Shell() {
    for (const auto &entry : menuBitmaps)
      if (entry.second)
        DeleteObject(entry.second);
    objects--;
  }
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
          if (n && DragQueryFileW(drop, 0xffffffff, nullptr, 0) == 1) {
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
    if ((flags & CMF_DEFAULTONLY) || path.empty() || last < first)
      return MAKE_HRESULT(SEVERITY_SUCCESS, 0, 0);
    commands = commandsFor(path);
    if (commands.empty())
      return MAKE_HRESULT(SEVERITY_SUCCESS, 0, 0);
    if (last - first + 1 < commands.size())
      return MAKE_HRESULT(SEVERITY_SUCCESS, 0, 0);
    const int size = menuIconSize();
    const HBITMAP rootBitmap = commandBitmap(-1, size);
    HBITMAP icons[std::size(menuIconIDs)]{};
    for (size_t i = 0; i < commands.size(); ++i)
      icons[i] = commandBitmap(commands[i], size);
    auto popup = CreatePopupMenu();
    if (!popup)
      return E_OUTOFMEMORY;
    for (UINT i = 0; i < commands.size(); i++) {
      MENUITEMINFOW item{};
      item.cbSize = sizeof(item);
      item.fMask = MIIM_ID | MIIM_STRING | MIIM_BITMAP;
      item.wID = first + i;
      item.dwTypeData = const_cast<LPWSTR>(commandTitle(commands[i]));
      item.hbmpItem = icons[i];
      if (!InsertMenuItemW(popup, i, TRUE, &item)) {
        DestroyMenu(popup);
        return E_FAIL;
      }
    }
    MENUITEMINFOW root{};
    root.cbSize = sizeof(root);
    root.fMask = MIIM_STRING | MIIM_SUBMENU | MIIM_BITMAP;
    root.dwTypeData = const_cast<LPWSTR>(L"MyGittu");
    root.hSubMenu = popup;
    root.hbmpItem = rootBitmap;
    if (!InsertMenuItemW(menu, pos, TRUE, &root)) {
      DestroyMenu(popup);
      return E_FAIL;
    }
    return MAKE_HRESULT(SEVERITY_SUCCESS, 0,
                        static_cast<USHORT>(commands.size()));
  } catch (...) {
    return E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE InvokeCommand(CMINVOKECOMMANDINFO *info) override
      try {
    if (!info || path.empty())
      return E_INVALIDARG;
    const auto command = reinterpret_cast<UINT_PTR>(info->lpVerb);
    if (command >= commands.size())
      return E_INVALIDARG;
    return launch(path, commands[command]);
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
    *index = kind <= 3 ? kind - 1 : kind + 7;
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
#include "modern.h"
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
    IUnknown *s = nullptr;
    if (kind == modernKind)
      s = static_cast<IExplorerCommand *>(new (std::nothrow) Modern());
    else
      s = static_cast<IShellExtInit *>(new (std::nothrow) Shell(kind));
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
  for (int id = 1; id <= 10; ++id) {
    auto candidate = baseID;
    candidate.Data4[7] = static_cast<BYTE>(id);
    if (clsid == candidate) {
      const int kind = id == 5 ? modernKind : id - (id > 5 ? 2 : 1);
      auto f = new (std::nothrow) Factory(kind);
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
