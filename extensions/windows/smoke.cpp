#include <windows.h>
#include <shlobj.h>
#include <iostream>
#include <chrono>
#include <string>

using GetClass = HRESULT(__stdcall *)(REFCLSID, REFIID, void **);
int wmain(int argc, wchar_t **argv) {
  if (argc < 3)
    return 2;
  CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
  auto dll = LoadLibraryW(argv[1]);
  if (!dll) {
    std::cerr << "LoadLibrary failed: " << GetLastError();
    return 3;
  }
  auto get =
      reinterpret_cast<GetClass>(GetProcAddress(dll, "DllGetClassObject"));
  auto unload = reinterpret_cast<HRESULT(__stdcall *)()>(
      GetProcAddress(dll, "DllCanUnloadNow"));
  if (!get || !unload)
    return 4;
  CLSID id = {0x29810e41,
              0x632a,
              0x4a9b,
              {0xa1, 0x90, 0x4d, 0x65, 0x93, 0x66, 0x81, 0x01}};
  for (int k = 1; k <= 3; k++) {
    id.Data4[7] = static_cast<BYTE>(k + 1);
    IClassFactory *factory = nullptr;
    if (FAILED(get(id, IID_IClassFactory, reinterpret_cast<void **>(&factory))))
      return 5;
    IShellIconOverlayIdentifier *overlay = nullptr;
    if (FAILED(factory->CreateInstance(nullptr, IID_IShellIconOverlayIdentifier,
                                       reinterpret_cast<void **>(&overlay))))
      return 6;
    wchar_t icon[32768];
    int index = -1;
    DWORD flags = 0;
    if (FAILED(overlay->GetOverlayInfo(icon, 32768, &index, &flags)) ||
        index != k - 1)
      return 7;
    auto start = std::chrono::steady_clock::now();
    auto result = overlay->IsMemberOf(argv[2], 0);
    auto ms = std::chrono::duration<double, std::milli>(
                  std::chrono::steady_clock::now() - start)
                  .count();
    std::cout << "overlay " << k << " match=" << (result == S_OK)
              << " latency_ms=" << ms << "\n";
    if (argc > 3 && (result == S_OK) != (k == _wtoi(argv[3])))
      return 10;
    overlay->Release();
    factory->Release();
  }
  id.Data4[7] = 1;
  IClassFactory *factory = nullptr;
  if (FAILED(get(id, IID_IClassFactory, reinterpret_cast<void **>(&factory))))
    return 8;
  IContextMenu *menu = nullptr;
  if (FAILED(factory->CreateInstance(nullptr, IID_IContextMenu,
                                     reinterpret_cast<void **>(&menu))))
    return 9;
  IShellExtInit *init = nullptr;
  if (FAILED(menu->QueryInterface(IID_IShellExtInit,
                                  reinterpret_cast<void **>(&init))))
    return 12;
  PIDLIST_ABSOLUTE pidl = nullptr;
  if (FAILED(SHParseDisplayName(argv[2], nullptr, &pidl, 0, nullptr)))
    return 13;
  auto initialized = init->Initialize(pidl, nullptr, nullptr);
  CoTaskMemFree(pidl);
  init->Release();
  if (FAILED(initialized))
    return 14;
  auto hmenu = CreatePopupMenu();
  auto result = menu->QueryContextMenu(hmenu, 0, 100, 110, 0);
  auto count = HRESULT_CODE(result);
  auto submenu = GetSubMenu(hmenu, 0);
  if (FAILED(result) || (count != 1 && count != 6) ||
      GetMenuItemCount(hmenu) != 1 || !submenu ||
      GetMenuItemCount(submenu) != count)
    return 15;
  MENUITEMINFOW item{};
  item.cbSize = sizeof(item);
  item.fMask = MIIM_BITMAP;
  if (!GetMenuItemInfoW(hmenu, 0, TRUE, &item) || !item.hbmpItem)
    return 17;
  if (argc > 4) {
    wchar_t label[128]{};
    GetMenuStringW(hmenu, 0, label, 128, MF_BYPOSITION);
    std::wstring expected = L"MyGittu";
    if (label != expected)
      return 16;
  }
  DestroyMenu(hmenu);
  menu->Release();
  factory->Release();
  id.Data4[7] = 5;
  if (FAILED(get(id, IID_IClassFactory, reinterpret_cast<void **>(&factory))))
    return 18;
  IExplorerCommand *modern = nullptr;
  if (FAILED(factory->CreateInstance(nullptr, IID_IExplorerCommand,
                                     reinterpret_cast<void **>(&modern))))
    return 19;
  EXPCMDFLAGS commandFlags{};
  if (FAILED(modern->GetFlags(&commandFlags)) ||
      commandFlags != ECF_HASSUBCOMMANDS)
    return 20;
  IShellItem *shellItem = nullptr;
  IShellItemArray *items = nullptr;
  if (FAILED(
          SHCreateItemFromParsingName(argv[2], nullptr, IID_IShellItem,
                                      reinterpret_cast<void **>(&shellItem))))
    return 21;
  if (FAILED(SHCreateShellItemArrayFromShellItem(
          shellItem, IID_IShellItemArray, reinterpret_cast<void **>(&items))))
    return 22;
  shellItem->Release();
  EXPCMDSTATE state{};
  if (FAILED(modern->GetState(items, TRUE, &state)) || state != ECS_ENABLED)
    return 23;
  IEnumExplorerCommand *enumerator = nullptr;
  if (FAILED(modern->EnumSubCommands(&enumerator)))
    return 24;
  int enabled = 0;
  IExplorerCommand *child = nullptr;
  while (enumerator->Next(1, &child, nullptr) == S_OK) {
    LPWSTR icon = nullptr;
    if (FAILED(child->GetIcon(items, &icon)) || !icon)
      return 25;
    CoTaskMemFree(icon);
    if (FAILED(child->GetState(items, TRUE, &state)))
      return 26;
    if (state == ECS_ENABLED)
      enabled++;
    child->Release();
  }
  if (enabled != count)
    return 27;
  IEnumExplorerCommand *clone = nullptr;
  if (FAILED(enumerator->Clone(&clone)))
    return 28;
  clone->Reset();
  if (clone->Next(1, &child, nullptr) != S_OK)
    return 29;
  child->Release();
  clone->Release();
  enumerator->Release();
  items->Release();
  modern->Release();
  factory->Release();
  if (unload() != S_OK)
    return 11;
  FreeLibrary(dll);
  CoUninitialize();
  return 0;
}
