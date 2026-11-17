# Windows 11 现代右键菜单

处理器原生 DLL 同时包含传统 IContextMenu 和现代 IExplorerCommand。新版菜单使用 CLSID `{29810E41-632A-4A9B-A190-4D6593668105}`，父命令通过 EnumSubCommands 返回单个 MyGittu 子菜单，支持文件、目录和目录背景。

## 构建及安装

新版菜单除 COM DLL 外还需要应用身份。实现依据 [Microsoft 官方集成说明](https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/integrate-packaged-app-with-file-explorer)。当前没有发布者签名证书，因此默认 EXE 安装包不宣称启用 Windows 11 现代菜单。CI 会构建 unsigned sparse package 验证清单；未签名包不能作为正式安装包部署。

在 Windows SDK 环境运行：

```powershell
.\packaging\windows\build-sparse.ps1 -Publisher 'CN=YourPublisher' -CertificatePath 'C:\certificates\publisher.pfx'
# PFX 密码通过 MYGITTU_CERT_PASSWORD 环境变量提供；Publisher 必须与证书 Subject 一致。
.\packaging\windows\register-modern.ps1 -PackagePath '.\dist\release\MyGittu-0.0.1-Shell.msix'
```

先安装对应本机架构的 MyGittu。证书必须在目标设备受信任。脚本不添加根证书、不修改 Developer Mode、不绕过签名验证。注册在当前用户身份执行，DLL 与 desktop.exe 通过 ExternalLocation 引用已安装目录。

卸载当前用户的菜单身份：

```powershell
Get-AppxPackage -Name Oranjekop.MyGittu.Shell | Remove-AppxPackage
```

先移除身份再卸载或移动应用目录。菜单注册更新后，按系统提示重新登录或重新启动 Explorer。传统菜单仍在“显示更多选项”中，现代身份注册不会删除传统菜单兼容入口。

## 验收与性能

独立 COM 宿主测试覆盖父菜单图标、六个仓库命令、普通目录克隆、现代命令状态、枚举克隆和 COM 引用释放；不能替代 Explorer 实机验证。正式签名后应在 Windows 11 26H2 分别验证 x64/ARM64、文件/文件夹/目录背景、中文/英文，以及升级和卸载恢复。

菜单不运行 Git、不遍历目录。IPC 每次最长 40ms；标记探测最多向上 64 层，目录判断最多缓存 1024 项、有效期两秒，供子命令复用。发现未打开的 Git 文件夹后，Agent 在后台建立状态缓存。现代 GetState 在不允许慢操作时返回 E_PENDING。覆盖图标继续使用内存快照。
