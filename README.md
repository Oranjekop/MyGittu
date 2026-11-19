<p align="center">
  <img src="internal/brand/icon-ui.png" width="88" height="88" alt="MyGittu 兔子图标">
</p>

# MyGittu

面向 Windows 和 macOS 的原生 Git 桌面客户端。支持文件预览、差异查看、区块暂存和提交历史，以及简体中文、英文和随系统切换的灰阶主题。

[下载安装包](https://github.com/Oranjekop/MyGittu/releases/latest) · [更新日志](CHANGELOG.md) · [反馈问题](https://github.com/Oranjekop/MyGittu/issues)

## 下载安装

先安装系统 Git，并用 `git --version` 确认可用。Windows 可用 [Git for Windows](https://git-scm.com/downloads/win)，macOS 可用 Xcode Command Line Tools 或已有的 Git。

| 平台 | 安装包 | 支持架构 |
| --- | --- | --- |
| Windows 10 或 11（64 位） | `MyGittu-<version>-Windows-Setup.exe` | 自动选择 x64 或 ARM64 |
| macOS 13 及以上 | `MyGittu-<version>-macOS.pkg` | Intel 和 Apple Silicon 通用包 |

发布页附有 `SHA256SUMS.txt`。安装引导使用中文，应用显示语言可在设置中切换。Windows 安装程序尚无 Authenticode 签名，macOS 安装包尚未公证，系统可能显示来源提示。

## 开始使用

1. 点击「打开仓库」，也可以拖入仓库目录，或按 `Ctrl+O`（macOS 为 `⌘O`）。
2. 点击顶部仓库名，从下拉菜单直接切换项目。同名仓库显示路径。按 `Ctrl+K` 或 `⌘K` 可按名称和路径搜索最近仓库。
3. 在「变更」页点选文件查看差异，勾选多项可批量暂存或取消暂存。窄窗会自动改用统一差异。
4. 在底部输入提交信息并提交暂存内容。草稿按仓库保存。点击提交人可修改仓库或全局 Git 身份。

## 工作区功能

| 页面 | 可以做什么 |
| --- | --- |
| 变更 | 比较暂存区和工作区，按文件或文本区块暂存，或恢复未暂存的改动。普通文本冲突可对照两侧内容，编辑结果后暂存。 |
| 文件 | 按名称或路径搜索目录树，预览图片和文本，查看包含重命名记录的文件历史、提交差异及逐行追溯。 |
| 历史 | 搜索提交、查看详情、反做提交或执行 Cherry-pick。撤销最近提交仅适用于单父提交且该提交未出现在远端跟踪分支上，撤销后保留修改和暂存内容。 |
| 分支 | 切换、合并或删除已合并的本地分支，比较本地或远端跟踪分支，管理远程仓库、上游跟踪和工作树。 |
| 储藏与标签 | 创建和应用 Stash，创建或删除本地标签。 |

顶部工具栏提供获取、仅快进拉取和推送，远程认证沿用系统 Git 的凭据助手或 SSH Agent。克隆入口位于左侧导航栏底部。

克隆支持 GitHub、Gitee、GitLab、Bitbucket 和通用 Git 地址。在「设置 → 克隆凭据」可保存多条 HTTPS、SSH 或 GitHub 登录配置，并在克隆时选择。手填的密码或令牌仅在当前应用会话有效。GitHub 浏览器授权由 Git Credential Manager 完成，授权后保存对应账号的配置即可用于克隆。

应用会定期检查稳定版更新，也可在设置中手动检查。发现新版后，可在应用内下载安装包，校验 SHA256 后打开系统安装器。下载支持查看进度和取消。

## 文件管理器集成

### Windows 资源管理器

在「设置 → 系统集成」中可开启或关闭资源管理器右键菜单和 Git 状态徽标，也可设置登录时启动 MyGittu。右键菜单按目录状态提供操作。Git 目录可查看变更、差异和历史，也可提交、拉取或推送。普通目录提供克隆。状态徽标区分干净、有修改和冲突，显示受 Windows 覆盖图标槽位限制。

Windows 11 新版菜单使用随安装包提供的自签名组件。在「设置 → 系统集成」首次启用右键菜单时，确认管理员授权后，应用会导入随安装包提供的发布证书并为当前用户注册新版菜单。传统入口位于「显示更多选项」。更改后可在同一页重启资源管理器以应用配置。[Windows 11 菜单部署说明](docs/runbooks/modern-menu.md)

### macOS Finder

默认安装包包含桌面客户端与后台服务。Finder Sync 扩展源码可用于签名开发构建。[原生集成指南](docs/runbooks/integration.md)

## 从源码构建

克隆本仓库，使用 `go.mod` 指定的 Go 工具链和系统 Git。

```sh
go test ./...
go vet ./...
```

Windows 本地运行：

```powershell
go build -o bin/agent.exe ./cmd/agent
go build -ldflags=-H=windowsgui -o bin/desktop.exe ./cmd/desktop
.\bin\desktop.exe
```

macOS 本地运行：

```sh
go build -o bin/agent ./cmd/agent
go build -o bin/desktop ./cmd/desktop
./bin/desktop
```

桌面端会启动同目录的 Agent，负责执行 Git 命令、监听仓库变化并维护状态缓存。[架构记录](docs/adr/001-architecture.md) · [IPC 协议](docs/protocol/v1.md)

打包输出至 `dist/release`。

| 平台 | 命令 | 额外依赖 |
| --- | --- | --- |
| Windows | `./packaging/windows/release.ps1` | Visual Studio C++ x64 和 ARM64 工具链、CMake、NSIS 3.13。可用 `MAKENSIS` 指定编译器。签名右键组件还需 Windows SDK 与签名证书。 |
| macOS | `bash packaging/darwin/release.sh` | Xcode Command Line Tools |

## 发布

更新 `VERSION` 和 [CHANGELOG.md](CHANGELOG.md)，提交并推送代码，再推送与版本文件一致的 `vX.Y.Z` 标签。GitHub Actions 将安装包上传到 Release 草稿，四种架构的安装检查通过后公开发布，并生成校验文件。

## 许可证

MyGittu 使用 [MIT License](LICENSE)。依赖及素材来源见[第三方声明](THIRD_PARTY_NOTICES.md)和 [SBOM](docs/sbom.cdx.json)。
