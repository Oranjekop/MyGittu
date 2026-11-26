#pragma once

#include <map>
#include <string_view>

namespace overlay_snapshot {

struct PathLess {
  using is_transparent = void;
  bool operator()(std::wstring_view a, std::wstring_view b) const {
    return CompareStringOrdinal(a.data(), static_cast<int>(a.size()), b.data(),
                                static_cast<int>(b.size()), TRUE) == CSTR_LESS_THAN;
  }
};

struct Snapshot {
  FILETIME updated{};
  DWORD size{};
  unsigned hidden{};
  std::vector<std::wstring> roots;
  std::map<std::wstring, int, PathLess> states;
};

static std::wstring wide(std::string_view text) {
  const auto length = static_cast<int>(text.size());
  int size = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(),
                                 length, nullptr, 0);
  std::wstring result(size, L'\0');
  if (size)
    MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(), length,
                        result.data(), size);
  return result;
}

static std::optional<Snapshot> read(const std::wstring &path,
                                    const WIN32_FIND_DATAW &entry) {
  constexpr DWORD maxSize = 32 * 1024 * 1024;
  if (entry.nFileSizeHigh || !entry.nFileSizeLow || entry.nFileSizeLow > maxSize)
    return std::nullopt;
  Handle file{CreateFileW(path.c_str(), GENERIC_READ,
                         FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
                         nullptr, OPEN_EXISTING, FILE_FLAG_SEQUENTIAL_SCAN, nullptr)};
  if (file.h == INVALID_HANDLE_VALUE)
    return std::nullopt;
  std::string buffer(entry.nFileSizeLow, '\0');
  DWORD count = 0;
  if (!ReadFile(file.h, buffer.data(), entry.nFileSizeLow, &count, nullptr) ||
      count != entry.nFileSizeLow || buffer.back() != '\n')
    return std::nullopt;
  std::string_view input(buffer);
  auto line = [&]() {
    const auto end = input.find('\n');
    const auto result = input.substr(0, end);
    input.remove_prefix(end == input.npos ? input.size() : end + 1);
    return result;
  };
  if (line() != "MYGITTU-OVERLAY-1")
    return std::nullopt;
  Snapshot snapshot;
  snapshot.updated = entry.ftLastWriteTime;
  snapshot.size = entry.nFileSizeLow;
  const auto mask = line();
  if (mask.empty() || mask.size() > 3)
    return std::nullopt;
  for (char digit : mask) {
    if (digit < '0' || digit > '9')
      return std::nullopt;
    snapshot.hidden = snapshot.hidden * 10 + digit - '0';
  }
  for (auto roots = line(); !roots.empty();) {
    const auto tab = roots.find('\t');
    auto root = wide(roots.substr(0, tab));
    if (root.empty())
      return std::nullopt;
    if (root.back() != L'\\')
      root += L'\\';
    snapshot.roots.push_back(std::move(root));
    roots.remove_prefix(tab == roots.npos ? roots.size() : tab + 1);
  }
  if (snapshot.roots.empty())
    return std::nullopt;
  constexpr std::string_view names[]{"clean", "modified", "conflicted", "added",
                                      "deleted", "untracked", "ignored", "behind"};
  while (!input.empty()) {
    const auto row = line();
    const auto tab = row.find('\t');
    if (tab == row.npos)
      return std::nullopt;
    const auto state = std::find(std::begin(names), std::end(names), row.substr(0, tab));
    auto key = wide(row.substr(tab + 1));
    if (state == std::end(names) || key.empty())
      return std::nullopt;
    snapshot.states.emplace(std::move(key), static_cast<int>(state - names) + 1);
  }
  return snapshot;
}

static std::optional<int> lookup(const std::wstring &path) {
  static const std::wstring directory = [] {
    const DWORD length = GetEnvironmentVariableW(L"APPDATA", nullptr, 0);
    if (!length)
      return std::wstring{};
    std::wstring result(length, L'\0');
    const auto copied = GetEnvironmentVariableW(L"APPDATA", result.data(), length);
    if (!copied || copied >= length)
      return std::wstring{};
    result.resize(copied);
    return result + L"\\MyGittu\\explorer-overlays-v1\\";
  }();
  if (directory.empty())
    return std::nullopt;
  struct Record {
    Snapshot snapshot;
    ULONGLONG seen{};
  };
  static std::mutex mutex;
  static std::map<std::wstring, Record> records;
  static ULONGLONG nextScan = 0;
  std::lock_guard<std::mutex> lock(mutex);
  const auto now = GetTickCount64();
  if (now >= nextScan) {
    nextScan = now + 200;
    WIN32_FIND_DATAW entry{};
    const auto search = FindFirstFileW((directory + L"*.overlay").c_str(), &entry);
    if (search != INVALID_HANDLE_VALUE) {
      struct SearchGuard {
        HANDLE handle;
        ~SearchGuard() { FindClose(handle); }
      } guard{search};
      do {
        if (entry.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)
          continue;
        auto found = records.find(entry.cFileName);
        if (found == records.end() ||
            CompareFileTime(&found->second.snapshot.updated, &entry.ftLastWriteTime) != 0 ||
            found->second.snapshot.size != entry.nFileSizeLow) {
          if (auto snapshot = read(directory + entry.cFileName, entry))
            found = records.insert_or_assign(entry.cFileName, Record{std::move(*snapshot), now}).first;
        }
        if (found != records.end())
          found->second.seen = now;
      } while (FindNextFileW(search, &entry));
      if (GetLastError() == ERROR_NO_MORE_FILES) {
        for (auto it = records.begin(); it != records.end();) {
          if (it->second.seen != now)
            it = records.erase(it);
          else
            ++it;
        }
      }
    } else if (GetLastError() == ERROR_FILE_NOT_FOUND ||
               GetLastError() == ERROR_PATH_NOT_FOUND) {
      records.clear();
    }
  }
  const Snapshot *best = nullptr;
  size_t prefix = 0;
  std::wstring_view relative;
  for (const auto &record : records) {
    for (const auto &root : record.second.snapshot.roots) {
      const bool exact = path.size() + 1 == root.size();
      const size_t length = exact ? root.size() - 1 : root.size();
      if (root.size() <= prefix || path.size() < length ||
          CompareStringOrdinal(path.data(), static_cast<int>(length), root.data(),
                               static_cast<int>(length), TRUE) != CSTR_EQUAL)
        continue;
      best = &record.second.snapshot;
      prefix = root.size();
      relative = exact || path.size() == length ? L"." : std::wstring_view(path).substr(length);
    }
  }
  if (!best)
    return std::nullopt;
  const PathLess less;
  const auto first = relative.substr(0, relative.find(L'\\'));
  if (!less(first, L".git") && !less(L".git", first))
    return 0;
  int state = 1;
  auto found = best->states.find(relative);
  if (found != best->states.end()) {
    state = found->second;
  } else {
    for (auto parent = relative.rfind(L'\\'); parent != relative.npos;
         parent = relative.rfind(L'\\')) {
      relative = relative.substr(0, parent);
      found = best->states.find(relative);
      if (found != best->states.end() && found->second == 7) {
        state = 7;
        break;
      }
    }
  }
  return best->hidden & (1u << (state - 1)) ? 0 : state;
}

} // namespace overlay_snapshot
