# ADR 001 共享 Go 核心与原生系统扩展

日期：2026-11-17。项目名称：MyGittu。状态：开发预览，系统集成门槛尚未全部通过。

从空目录独立实现。Go Agent 持有仓库注册、Git 服务与不可变快照；MyGo UI 只使用 IPC；C++ COM 与 Swift Finder Sync 负责平台回调。没有复制或移植 TortoiseGit 实现。

MyGo 固定 v0.9.2，提交 `25ce62f4d357cd6d29da04300e3fbc352d79daa4`。Go 固定 1.27.4（MyGo 最低要求 1.27.3）。系统 Git 支持目标 2.40+；本机已测 2.56.0.windows.4，其余版本待矩阵验证。TortoiseGit 仅作为功能参照，记录参照 HEAD `c84d0ce4724379a672f4efa10e1625ec0c9e4676`，不作为构建依赖。

Git 通过参数数组运行，禁用终端密码提示和 fsmonitor，限制执行时间与输出大小。读操作中的 Diff 禁用外部 diff 与 textconv；仓库写操作串行，等待可取消。保留系统 Git 的认证和 Hook 行为。

Windows 使用当前用户 SID 限定的 Named Pipe，并拒绝远程客户端；macOS 使用 App Group 下 0700 目录和 0600 Unix socket，并校验 peer UID。JSON 帧以 4 字节大端长度开头，协议版本为 1；最多 32 个并发连接，单帧最多 8MiB。暴露的方法都有独立参数校验，不提供任意命令执行。

缓存先使用不可变内存快照和 generation。事件通知只失效与调度，查询不调用 Git。扫描并发为 1，监听目录有上限。当前 fsnotify 的 macOS kqueue 实现是显式适配选择；大规模部署前仍需评估 FSEvents 的成本。

Windows Overlay 保留 3 个状态；现代 Windows 11 IExplorerCommand 尚未实现。Finder 只管理注册仓库。对签名、沙盒与扩展加载的实际验证是发布前置条件，交叉编译 Go 程序不能替代这些测试。

官方资料：[MyGo](https://github.com/egoist/mygo/tree/25ce62f4d357cd6d29da04300e3fbc352d79daa4)、[Windows Overlay](https://learn.microsoft.com/en-us/windows/win32/shell/how-to-implement-icon-overlay-handlers)、[Finder Sync](https://developer.apple.com/library/archive/documentation/General/Conceptual/ExtensibilityPG/Finder.html)。
