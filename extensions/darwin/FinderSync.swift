import Cocoa
import FinderSync
import Darwin

// A serial utility queue owns socket work; Finder callbacks never wait for IPC.
final class AgentClient {
    let socketURL: URL
    init(directory: URL) { socketURL = directory.appendingPathComponent("agent.sock") }
    func call(_ method: String, params: [String: Any] = [:]) throws -> Any {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { throw POSIXError(.EIO) }
        defer { close(fd) }
        var timeout = timeval(tv_sec: 0, tv_usec: 40_000)
        setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout.size(ofValue: timeout)))
        setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, socklen_t(MemoryLayout.size(ofValue: timeout)))
        var noSignal: Int32 = 1
        setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &noSignal, socklen_t(MemoryLayout.size(ofValue: noSignal)))
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        address.sun_len = UInt8(MemoryLayout<sockaddr_un>.size)
        let path = Array(socketURL.path.utf8CString)
        guard path.count <= MemoryLayout.size(ofValue: address.sun_path) else { throw POSIXError(.ENAMETOOLONG) }
        withUnsafeMutableBytes(of: &address.sun_path) { dest in path.withUnsafeBytes { dest.copyBytes(from: $0) } }
        let connected = withUnsafePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { Darwin.connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        guard connected == 0 else { throw POSIXError(.ECONNREFUSED) }
        let id = UUID().uuidString
        let payload = try JSONSerialization.data(withJSONObject: ["version": 1, "request_id": id, "method": method, "params": params])
        var length = UInt32(payload.count).bigEndian
        var frame = withUnsafeBytes(of: &length) { Data($0) }; frame.append(payload)
        try frame.withUnsafeBytes { bytes in
            var offset = 0
            while offset < bytes.count {
                let n = write(fd, bytes.baseAddress!.advanced(by: offset), bytes.count - offset)
                guard n > 0 else { throw POSIXError(.EIO) }; offset += n
            }
        }
        func receive(_ count: Int) throws -> Data {
            var data = Data(count: count)
            try data.withUnsafeMutableBytes { bytes in
                var offset = 0
                while offset < count {
                    let n = read(fd, bytes.baseAddress!.advanced(by: offset), count - offset)
                    guard n > 0 else { throw POSIXError(.EIO) }; offset += n
                }
            }; return data
        }
        let header = try receive(4)
        let size = header.reduce(UInt32(0)) { ($0 << 8) | UInt32($1) }
        guard size > 0 && size <= 8 * 1024 * 1024 else { throw POSIXError(.EMSGSIZE) }
        guard let response = try JSONSerialization.jsonObject(with: receive(Int(size))) as? [String: Any],
              response["request_id"] as? String == id, response["version"] as? Int == 1,
              response["ok"] as? Bool == true else { throw POSIXError(.EPROTO) }
        return response["result"] ?? NSNull()
    }
}

@objc(FinderSync)
final class FinderSync: FIFinderSync {
    private let queue = DispatchQueue(label: "dev.mygittu.finder", qos: .utility)
    private var client: AgentClient?
    private var timer: Timer?
    private var visible = Set<URL>()
    private var pending = Set<URL>()
    private var directory: URL?
    private var language = Locale.preferredLanguages.first?.hasPrefix("zh") == true ? "zh-CN" : "en"

    override init() {
        super.init()
        configureBadges()
        guard let group = Bundle.main.object(forInfoDictionaryKey: "MyGittuAppGroup") as? String,
              let directory = FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: group) else { return }
        self.directory = directory
        client = AgentClient(directory: directory)
        updateRoots()
        timer = Timer.scheduledTimer(withTimeInterval: 2, repeats: true) { [weak self] _ in
            self?.updateRoots()
            for url in self?.visible ?? [] { self?.requestBadgeIdentifier(for: url) }
        }
    }
    private func configureBadges() {
        let controller = FIFinderSyncController.default()
        for (state, symbol, zh, en) in [("clean", "checkmark.circle.fill", "无变更", "Clean"), ("modified", "pencil.circle.fill", "有变更", "Modified"), ("conflicted", "exclamationmark.circle.fill", "有冲突", "Conflicted")] {
            let label = language == "en" ? en : zh
            if let image = NSImage(systemSymbolName: symbol, accessibilityDescription: label) { controller.setBadgeImage(image, label: label, forBadgeIdentifier: state) }
        }
    }
    private func updateRoots() {
        guard let client = client else { return }
        queue.async {
            if let prefs = try? client.call("settings.get") as? [String: Any], let language = prefs["language"] as? String {
                DispatchQueue.main.async { if self.language != language { self.language = language; self.configureBadges() } }
            }
            guard let repos = try? client.call("repo.list") as? [[String: Any]] else { return }
            let roots = Set(repos.compactMap { ($0["root"] as? String).map { URL(fileURLWithPath: $0) } })
            DispatchQueue.main.async { FIFinderSyncController.default().directoryURLs = roots }
        }
    }
    override func requestBadgeIdentifier(for url: URL) {
        guard let client = client else { return }
        // Bound memory and refresh work for long Finder sessions.
        if visible.count >= 512 { visible.removeAll() }; visible.insert(url)
        guard pending.count < 512 && !pending.contains(url) else { return }
        pending.insert(url)
        queue.async {
            let response = try? client.call("overlay.lookup", params: ["path": url.path]) as? [String: Any]
            var badge = response?["state"] as? String ?? ""
            if ["added", "deleted", "untracked"].contains(badge) { badge = "modified" }
            if !["clean", "modified", "conflicted"].contains(badge) { badge = "" }
            DispatchQueue.main.async { self.pending.remove(url); FIFinderSyncController.default().setBadgeIdentifier(badge, for: url) }
        }
    }
    override func endObservingDirectory(at url: URL) { visible = visible.filter { !$0.path.hasPrefix(url.path + "/") } }
    override func menu(for menuKind: FIMenuKind) -> NSMenu? {
        let menu = NSMenu(title: "MyGittu")
        for (action, zh, en) in [("status", "状态", "Status"), ("commit", "提交", "Commit"), ("log", "历史", "Log"), ("diff", "差异", "Diff")] {
            let item = NSMenuItem(title: "MyGittu · " + (language == "en" ? en : zh), action: #selector(openDesktop(_:)), keyEquivalent: "")
            item.target = self; item.representedObject = action; menu.addItem(item)
        }; return menu
    }
    @objc private func openDesktop(_ sender: NSMenuItem) {
        guard let action = sender.representedObject as? String, ["status", "commit", "log", "diff"].contains(action),
              let url = FIFinderSyncController.default().selectedItemURLs()?.first ?? FIFinderSyncController.default().targetedURL() else { return }
        // The host bundle is exactly two bundle ancestors above PlugIns/*.appex.
        let host = Bundle.main.bundleURL.deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let config = NSWorkspace.OpenConfiguration(); config.createsNewApplicationInstance = true
        config.arguments = ["--action", action, "--path", url.path]
        NSWorkspace.shared.openApplication(at: host, configuration: config) { _, _ in }
    }
}
