# 原生集成验证

所有注册和卸载测试优先在隔离 Windows VM 完成。生产机器的 Explorer 不作为测试进程。

## Windows

1. 运行 Go 测试与 `packaging/windows/build.ps1`，将 DLL 与 desktop.exe 放在 bin 同目录。
2. 用 `extensions/windows/smoke.cpp` 构建独立测试程序；先验证 DLL 可加载、三个 Overlay 工厂和菜单工厂能创建，释放后 DllCanUnloadNow 返回 S_OK。
3. 启动 Agent，注册临时仓库并刷新状态，再运行 `shell-smoke.exe <DLL绝对路径> <仓库路径> <预期映射>`。映射为 0=None、1=Clean、2=Modified、3=Conflict。
4. 关闭 Agent 再重复查询，必须迅速返回 None。畸形报文、超时与缓存过期测试不能使宿主崩溃。
5. 在 VM 注册并重新启动 Explorer，验证目录/文件/背景菜单、含空格/中文路径、多选、Commit/Log/Diff 和三个图标。多选只打开第一个路径所在仓库。
6. 使用 `register.ps1 -Uninstall`；以管理员身份运行才会同时移除 HKLM Overlay 注册。重启 Explorer 后再替换/删除仍被加载的 DLL。

可用独立 LLVM MinGW 编译源码；本机使用 20261104 版本，开启 C++17、Unicode、x64、静态 C++ 运行库和 `-Werror`。正式安装包优先 MSVC，仍需对应 CI 验证。

## macOS

1. 设置 `SIGN_IDENTITY` 为 Developer ID Application 身份，`MYGITTU_APP_GROUP` 为 `TEAMID.mygittu`。Team ID 必须与证书一致。发布工作流从 `MYGITTU_MACOS_CERT_BASE64`、`MYGITTU_MACOS_CERT_PASSWORD` 读取 P12，从仓库变量 `MYGITTU_MACOS_SIGN_IDENTITY`、`MYGITTU_MACOS_APP_GROUP` 读取身份与分组。
2. 运行 `bash packaging/darwin/release.sh` 生成包含 Universal Finder 扩展的 PKG。仅构建签名的 `bin/MyGittu.app` 时运行 `bash packaging/darwin/build.sh`。
3. 安装 PKG，启动 MyGittu 并添加仓库。在系统设置中搜索「Finder 扩展」，启用「MyGittu Finder」。
4. 在已注册仓库中验证状态、提交、历史、差异右键命令，以及干净、修改、冲突徽章。再验证 Agent 退出和重启、Finder 重启、扩展重载和多个仓库。
5. IPC 位于 App Group 的 `ipc` 子目录。核查 0700 目录、0600 socket、peer UID 和沙盒日志。仓库注册、设置和草稿仍存放在原用户配置目录。

## 性能与恢复

分别建立 1 万/10 万真实文件仓库；记录注册、全量扫描、增量刷新、IPC 缓存查询和 UI 滚动的 P50/P95、CPU、峰值内存与 OS 句柄数。独立缓存微基准不能代替端到端性能验收。

Agent 重启时读取仓库注册记录并重建快照。缓存缺失时返回 Unknown。桌面端启动时会启动缺失的 Agent，也可在「设置 → 系统集成」开启登录时启动。应在故障注入测试中验证取消 Git 操作后 Hook 和 SSH 子进程的退出情况。
