package i18n

// Catalog contains product text only. Git output, paths, branch names and user
// content remain unchanged. IDs are stable across language switches.
type Message struct{ ZH, EN string }

var Catalog = map[string]Message{
	"nav.workspace":      {"工作区", "Workspace"},
	"repo.search":        {"按名称或路径搜索", "Search by name or path"},
	"repo.forget":        {"从最近仓库移除", "Remove from recent repositories"},
	"repo.noMatches":     {"没有匹配的仓库", "No matching repositories"},
	"nav.files":          {"文件", "Files"},
	"files.empty":        {"没有项目文件", "No project files"},
	"files.select":       {"选择文件以预览", "Select a file to preview"},
	"commit.running":     {"正在提交…", "Committing…"},
	"clone.running":      {"正在克隆…", "Cloning…"},
	"update.install":     {"下载并安装 %s", "Download and install %s"},
	"update.downloading": {"正在下载安装包…", "Downloading installer…"},
	"close":              {"关闭", "Close"},
	"changes.nav":        {"变更", "Changes"}, "diff.binary": {"二进制文件，无法显示文本差异", "Binary file; text comparison unavailable"},
	"update.title": {"软件更新", "Software updates"}, "update.check": {"检查更新", "Check for updates"}, "update.current": {"当前已是最新版本", "You're up to date"}, "update.available": {"MyGittu %s 可用", "MyGittu %s is available"},
	"clone.title": {"克隆 Git 仓库", "Clone Git repository"}, "clone.url": {"仓库地址", "Repository URL"}, "clone.destination": {"目标文件夹（绝对路径）", "Destination folder (absolute path)"}, "clone.run": {"开始克隆", "Clone"},
	"identity.edit": {"设置提交人…", "Set commit identity…"},
	"repo.drop":     {"选择或拖入 Git 仓库文件夹", "Choose or drop a Git repository folder"}, "repo.recent": {"最近仓库", "Recent repositories"},
	"identity.title": {"提交人信息", "Commit identity"}, "identity.local": {"当前仓库", "This repository"}, "identity.global": {"全局默认", "Global default"}, "identity.name": {"姓名", "Name"}, "identity.email": {"邮箱", "Email"}, "identity.save": {"保存提交人信息", "Save identity"}, "identity.saved": {"已保存", "Saved"}, "identity.future": {"用于后续提交，不修改已有提交记录", "Applies to future commits; existing commits remain unchanged"},
	"repo.choose": {"选择目录…", "Choose folder…"}, "repo.dialog": {"选择 Git 仓库", "Select Git repository"}, "repo.open": {"打开仓库", "Open repository"}, "repo.remove": {"移除仓库", "Remove repository"}, "repo.empty": {"打开一个 Git 仓库开始使用", "Open a Git repository to get started"},
	"nav.commit": {"提交", "Commit"}, "nav.history": {"历史", "History"}, "nav.diff": {"差异", "Diff"}, "nav.branches": {"分支", "Branches"}, "nav.settings": {"设置", "Settings"}, "refresh": {"刷新", "Refresh"}, "busy": {"处理中…", "Working…"}, "error": {"操作失败", "Operation failed"},
	"changes": {"%d 个变更", "%d changes"}, "stage.all": {"全部暂存", "Stage all"}, "diff.staged": {"暂存区差异", "Staged diff"}, "file": {"文件", "File"}, "index": {"暂存区", "Index"}, "worktree": {"工作区", "Worktree"}, "actions": {"操作", "Actions"}, "stage": {"暂存", "Stage"}, "unstage": {"取消暂存", "Unstage"}, "diff.view": {"查看差异", "View diff"}, "diff.all": {"全部差异", "All changes"}, "diff.empty": {"没有可显示的差异（未跟踪文件请先暂存）", "No diff to display (stage untracked files first)"}, "commit.message": {"提交信息", "Commit message"}, "commit.submit": {"提交暂存内容", "Commit staged changes"},
	"history.search": {"搜索提交信息", "Search commit messages"}, "search": {"搜索", "Search"}, "subject": {"说明", "Subject"}, "author": {"作者", "Author"}, "date": {"日期", "Date"}, "commit.details": {"提交详情", "Commit details"}, "previous": {"上一页", "Previous"}, "next": {"下一页", "Next"}, "history.empty": {"暂无提交", "No commits"},
	"branch.name": {"分支名称", "Branch name"}, "branch.create": {"创建", "Create"}, "branch.switch": {"切换", "Switch"}, "branch.delete": {"删除已合并分支", "Delete merged branch"}, "branch.merge": {"合并到当前分支", "Merge into current"},
	"settings.language": {"语言", "Language"}, "settings.theme": {"主题", "Theme"}, "theme.system": {"跟随系统", "System"}, "theme.light": {"浅色", "Light"}, "theme.dark": {"深色", "Dark"},
	"settings.credentials": {"Git 凭据由系统 Git credential helper 管理", "Credentials use the system Git credential helper"}, "settings.cache": {"文件变更后刷新缓存，每 30 秒校验一次", "Refresh on file changes; reconcile every 30 seconds"}, "settings.integration": {"Windows 覆盖图标和 Finder 扩展需要单独安装启用", "Explorer overlays and the Finder extension require separate installation"},
	"remote.fetch": {"获取", "Fetch"}, "remote.pull": {"拉取（仅快进）", "Pull (fast-forward)"}, "remote.push": {"推送", "Push"},
	"state.clean": {"—", "—"}, "state.modified": {"已修改", "Modified"}, "state.added": {"已新增", "Added"}, "state.deleted": {"已删除", "Deleted"}, "state.untracked": {"未跟踪", "Untracked"}, "state.ignored": {"已忽略", "Ignored"}, "state.conflicted": {"有冲突", "Conflicted"}, "state.unknown": {"未知", "Unknown"}, "unsupported": {"不支持的操作", "Unsupported action"},
}

func Text(language, id string) string {
	m, ok := Catalog[id]
	if !ok {
		return id
	}
	if language == "en" {
		return m.EN
	}
	return m.ZH
}
