// Included by shell.cpp: shares fixed-path launch and bounded shell helpers.
#include <shlguid.h>
#include <servprov.h>

static HRESULT copyText(const std::wstring &text, LPWSTR *out) {
  if (!out)
    return E_POINTER;
  *out = nullptr;
  auto bytes = (text.size() + 1) * sizeof(wchar_t);
  *out = static_cast<LPWSTR>(CoTaskMemAlloc(bytes));
  if (!*out)
    return E_OUTOFMEMORY;
  memcpy(*out, text.c_str(), bytes);
  return S_OK;
}
class Modern;
class CommandEnum final : public IEnumExplorerCommand {
  std::atomic<ULONG> refs{1};
  std::vector<IExplorerCommand *> entries;
  size_t position = 0;

public:
  explicit CommandEnum(std::vector<IExplorerCommand *> values)
      : entries(std::move(values)) {
    objects++;
  }
  ~CommandEnum() {
    for (auto item : entries)
      item->Release();
    objects--;
  }
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **out) override {
    if (!out)
      return E_POINTER;
    *out = nullptr;
    if (iid != IID_IUnknown && iid != IID_IEnumExplorerCommand)
      return E_NOINTERFACE;
    *out = this;
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
  HRESULT STDMETHODCALLTYPE Next(ULONG count, IExplorerCommand **out,
                                 ULONG *fetched) override {
    if (!out || (!fetched && count != 1))
      return E_POINTER;
    ULONG n = 0;
    while (n < count && position < entries.size()) {
      out[n] = entries[position++];
      out[n++]->AddRef();
    }
    if (fetched)
      *fetched = n;
    return n == count ? S_OK : S_FALSE;
  }
  HRESULT STDMETHODCALLTYPE Skip(ULONG count) override {
    auto n = entries.size() - position;
    position += std::min(static_cast<size_t>(count), n);
    return count <= n ? S_OK : S_FALSE;
  }
  HRESULT STDMETHODCALLTYPE Reset() override {
    position = 0;
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE Clone(IEnumExplorerCommand **out) override {
    if (!out)
      return E_POINTER;
    *out = nullptr;
    try {
      auto copy = entries;
      auto clone = new CommandEnum(std::move(copy));
      for (auto item : clone->entries)
        item->AddRef();
      clone->position = position;
      *out = clone;
      return S_OK;
    } catch (...) {
      return E_OUTOFMEMORY;
    }
  }
};
class Modern final : public IExplorerCommand, public IObjectWithSite {
  std::atomic<ULONG> refs{1};
  IUnknown *site = nullptr;
  std::wstring path;
  int action;
  std::wstring selected(IShellItemArray *items) {
    IShellItem *item = nullptr;
    if (items) {
      DWORD count = 0;
      if (SUCCEEDED(items->GetCount(&count)) && count != 1)
        return L"";
      items->GetItemAt(0, &item);
    } else if (site) {
      IServiceProvider *provider = nullptr;
      if (SUCCEEDED(site->QueryInterface(
              IID_IServiceProvider, reinterpret_cast<void **>(&provider)))) {
        IFolderView *view = nullptr;
        provider->QueryService(SID_SFolderView, IID_IFolderView,
                               reinterpret_cast<void **>(&view));
        if (!view) {
          IShellBrowser *browser = nullptr;
          if (SUCCEEDED(provider->QueryService(
                  SID_STopLevelBrowser, IID_IShellBrowser,
                  reinterpret_cast<void **>(&browser)))) {
            IShellView *shellView = nullptr;
            if (SUCCEEDED(browser->QueryActiveShellView(&shellView))) {
              shellView->QueryInterface(IID_IFolderView,
                                        reinterpret_cast<void **>(&view));
              shellView->Release();
            }
            browser->Release();
          }
        }
        if (view) {
          view->GetFolder(IID_IShellItem, reinterpret_cast<void **>(&item));
          view->Release();
        }
        provider->Release();
      }
    }
    if (!item)
      return path;
    LPWSTR name = nullptr;
    auto hr = item->GetDisplayName(SIGDN_FILESYSPATH, &name);
    item->Release();
    if (FAILED(hr))
      return L"";
    std::wstring result = name;
    CoTaskMemFree(name);
    return result;
  }

public:
  explicit Modern(int value = -1, std::wstring location = L"")
      : path(std::move(location)), action(value) {
    objects++;
  }
  ~Modern() {
    if (site)
      site->Release();
    objects--;
  }
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **out) override {
    if (!out)
      return E_POINTER;
    *out = nullptr;
    if (iid == IID_IUnknown || iid == IID_IExplorerCommand)
      *out = static_cast<IExplorerCommand *>(this);
    else if (iid == IID_IObjectWithSite)
      *out = static_cast<IObjectWithSite *>(this);
    else
      return E_NOINTERFACE;
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
  HRESULT STDMETHODCALLTYPE SetSite(IUnknown *value) override {
    if (value)
      value->AddRef();
    if (site)
      site->Release();
    site = value;
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE GetSite(REFIID iid, void **out) override {
    if (!out)
      return E_POINTER;
    *out = nullptr;
    return site ? site->QueryInterface(iid, out) : E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE GetTitle(IShellItemArray *, LPWSTR *out) override
      try {
    return copyText(action < 0 ? L"MyGittu" : commandTitle(action), out);
  } catch (...) {
    return E_OUTOFMEMORY;
  }
  HRESULT STDMETHODCALLTYPE GetIcon(IShellItemArray *, LPWSTR *out) override
      try {
    return copyText(modulePath() + L",-104", out);
  } catch (...) {
    return E_OUTOFMEMORY;
  }
  HRESULT STDMETHODCALLTYPE GetToolTip(IShellItemArray *,
                                       LPWSTR *out) override {
    if (!out)
      return E_POINTER;
    *out = nullptr;
    return E_NOTIMPL;
  }
  HRESULT STDMETHODCALLTYPE GetCanonicalName(GUID *out) override {
    if (!out)
      return E_POINTER;
    *out = ids[0];
    out->Data4[7] = static_cast<BYTE>(action + 16);
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE GetState(IShellItemArray *items, BOOL slow,
                                     EXPCMDSTATE *state) override try {
    if (!state)
      return E_POINTER;
    auto location = selected(items);
    if (location.empty()) {
      *state = ECS_HIDDEN;
      return S_OK;
    }
    path = location;
    if (action < 0) {
      *state = ECS_ENABLED;
      return S_OK;
    }
    if (!slow) {
      *state = ECS_DISABLED;
      return E_PENDING;
    }
    bool repo = repositoryPath(location);
    *state = ((action == 6) != repo) ? ECS_ENABLED : ECS_HIDDEN;
    return S_OK;
  } catch (...) {
    return E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE Invoke(IShellItemArray *items, IBindCtx *) override
      try {
    return launch(selected(items), action);
  } catch (...) {
    return E_FAIL;
  }
  HRESULT STDMETHODCALLTYPE GetFlags(EXPCMDFLAGS *out) override {
    if (!out)
      return E_POINTER;
    *out = action < 0 ? ECF_HASSUBCOMMANDS : ECF_DEFAULT;
    return S_OK;
  }
  HRESULT STDMETHODCALLTYPE EnumSubCommands(IEnumExplorerCommand **out) override
      try {
    if (!out)
      return E_POINTER;
    *out = nullptr;
    if (action >= 0)
      return E_NOTIMPL;
    std::vector<IExplorerCommand *> commands;
    try {
      for (int value : {0, 4, 1, 5, 2, 3, 6})
        commands.push_back(new Modern(value, path));
      *out = new CommandEnum(std::move(commands));
      return S_OK;
    } catch (...) {
      for (auto item : commands)
        item->Release();
      return E_OUTOFMEMORY;
    }
  } catch (...) {
    return E_FAIL;
  }
};
