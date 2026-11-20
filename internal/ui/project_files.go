package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"mygittu/internal/app"
	"mygittu/internal/ipc"
)

const projectFilesPage = 7

type projectEntry struct {
	path string
	dir  bool
}

func (d *Desktop) rebuildProjectTree() {
	selected := d.selectedProjectPath()
	d.projectTree = d.projectTree[:0]
	query := strings.ToLower(strings.TrimSpace(d.projectSearch))
	children := make(map[string][]projectEntry)
	seen := make(map[string]bool)
	for _, file := range d.projectFiles {
		if !strings.Contains(strings.ToLower(file), query) {
			continue
		}
		parts := strings.Split(file, "/")
		var dir string
		hidden := false
		for _, part := range parts[:len(parts)-1] {
			parent := dir
			dir = path.Join(parent, part)
			if !seen[dir] {
				children[parent] = append(children[parent], projectEntry{dir, true})
				seen[dir] = true
			}
			if query == "" && !d.projectExpanded[dir] {
				hidden = true
				break
			}
		}
		if !hidden {
			children[dir] = append(children[dir], projectEntry{path: file})
		}
	}
	var appendChildren func(string)
	appendChildren = func(parent string) {
		entries := children[parent]
		slices.SortFunc(entries, func(a, b projectEntry) int {
			if a.dir != b.dir {
				if a.dir {
					return -1
				}
				return 1
			}
			if order := strings.Compare(strings.ToLower(path.Base(a.path)), strings.ToLower(path.Base(b.path))); order != 0 {
				return order
			}
			return strings.Compare(a.path, b.path)
		})
		for _, entry := range entries {
			d.projectTree = append(d.projectTree, entry)
			if entry.dir {
				appendChildren(entry.path)
			}
		}
	}
	appendChildren("")
	d.projectFileRow = slices.IndexFunc(d.projectTree, func(entry projectEntry) bool { return !entry.dir && entry.path == selected })
}

func (d *Desktop) selectedProjectPath() string {
	if row := d.projectFileRow; row >= 0 && row < len(d.projectTree) && !d.projectTree[row].dir {
		return d.projectTree[row].path
	}
	return ""
}

func (d *Desktop) showProjectFiles(c *native.Context, t *native.Theme) {
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		native.Column(c).Width(342).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			before := d.projectSearch
			native.TextInput(c, &d.projectSearch).Placeholder(d.text("files.search")).FillWidth().Disabled(d.busy)
			if before != d.projectSearch {
				d.rebuildProjectTree()
			}
			empty := len(d.projectTree) == 0 && !d.busy && d.err == ""
			if !empty {
				native.Text(c, d.text("nav.files")).Bold().TextColor(t.TextMuted).PaddingStart(controlSpacing)
				native.Divider(c)
			}
			d.projectFileList.Selected = &d.projectFileRow
			d.projectFileList.RoundRows = true
			previous := d.projectFileRow
			list := scrollAtPanelEdge(native.List(c, &d.projectFileList, len(d.projectTree), func(row int) {
				entry := d.projectTree[row]
				name := path.Base(entry.path)
				native.Row(c).FillWidth().MinWidth(0).Height(controlHeight(c)).Gap(detailSpacing).AlignItems(native.Center).
					PaddingStart(controlSpacing + float32(strings.Count(entry.path, "/")*12)).
					PaddingEnd(controlSpacing).Tooltip(entry.path).Children(func() {
					icon := "open"
					if entry.dir {
						arrow := "down"
						if strings.TrimSpace(d.projectSearch) == "" && !d.projectExpanded[entry.path] {
							icon, arrow = "folder", "right"
						}
						native.Icon(c, icons[arrow]).Width(12).Height(12)
					} else {
						native.Box(c).Width(12)
						icon = projectFileIcon(name)
					}
					native.Icon(c, icons[icon]).Width(15).Height(15).
						TextColor(fileIconColor(icon, t.Dark != (row == d.projectFileRow)))
					native.Text(c, name).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				})
			})).MinHeight(0).Clip().Disabled(d.busy)
			message := d.text("files.empty")
			if len(d.projectFiles) > 0 {
				message = d.copy("没有匹配的文件", "No matching files")
			}
			if empty {
				list.Hidden(true)
				emptyState(c, emptyPreviewArt, 88, message).Grow(1)
			} else {
				list.Grow(1)
			}
			if row := d.projectFileRow; row != previous && row >= 0 && row < len(d.projectTree) && !d.busy {
				entry := d.projectTree[row]
				if entry.dir {
					if strings.TrimSpace(d.projectSearch) != "" {
						d.projectFileRow = previous
						return
					}
					if d.projectExpanded == nil {
						d.projectExpanded = make(map[string]bool)
					}
					d.projectExpanded[entry.path] = !d.projectExpanded[entry.path]
					if !d.projectExpanded[entry.path] {
						delete(d.projectExpanded, entry.path)
					}
					d.saveProjectFolders()
					d.projectFileRow = previous
					d.rebuildProjectTree()
					return
				}
				d.openProjectTab(entry.path)
			}
		})
		native.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			d.showProjectTabs(c)
			native.Column(c).Grow(1).MinHeight(0).FillWidth().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
				d.showProjectPreview(c, t)
			})
		})
	})
}

func (d *Desktop) showProjectPreview(c *native.Context, t *native.Theme) {
	filename := d.projectShownPath
	if filename == "" && d.projectLoad == nil {
		filename = d.selectedProjectPath()
	}
	switch {
	case filename == "" && d.projectLoad != nil:
		native.Text(c, d.text("busy")).TextColor(t.TextMuted)
	case filename == "":
		if !d.busy && d.projectLoadError == "" && d.err == "" {
			emptyState(c, emptyPreviewArt, 128, d.text("files.select")).Grow(1)
		}
	default:
		if !d.projectPreview.Binary {
			native.Text(c, filename).Bold().FillWidth().MinWidth(0).SingleLine().Ellipsis("…").Tooltip(filename)
		}
		if d.projectBitmap == nil && !d.projectPreview.Binary && (d.projectShownPath != "" || d.projectLoadError == "") {
			d.showCodePreview(c, filename)
		} else {
			native.Row(c).FillWidth().Gap(controlSpacing).Wrap().Children(func() {
				d.projectFileActions(c, filename)
			})
			if d.projectBitmap != nil {
				native.Image(c, d.projectBitmap).Grow(1).MinHeight(0).FillWidth().Fit(native.Contain).Label(filename)
			} else if d.projectPreview.Binary {
				emptyState(c, emptyPreviewArt, 128, d.text("diff.binary")).Grow(1)
			}
		}
	}
	if d.projectLoadError != "" {
		native.Text(c, d.projectLoadError).FillWidth().TextColor(t.Danger)
	}
}

func (d *Desktop) projectFileActions(c *native.Context, filename string) {
	if failed := d.projectFailedPath; d.projectLoadError != "" && failed != "" && IconButton(c, d.text("refresh"), "refresh", false).Disabled(d.busy).Clicked() {
		d.loadProjectPreview(failed)
	}
	if IconButton(c, d.text("file.history"), "history", false).Disabled(d.busy).Clicked() {
		d.openFileHistory(filename)
	}
	label := d.text("files.openExternal")
	if IconButton(c, "", "external", false).Label(label).Tooltip(label).
		Disabled(d.busy || !filepath.IsAbs(d.path)).Clicked() {
		d.openProjectFileExternally(filename)
	}
}

func (d *Desktop) openProjectFileExternally(filename string) {
	rootPath := d.path
	d.task(func(ctx context.Context) (func(), error) {
		name, err := filepath.Localize(filename)
		if err != nil {
			return nil, err
		}
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		info, err := root.Stat(name)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("not a regular file: %s", filename)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, mygo.Shell.OpenPath(filepath.Join(rootPath, name))
	})
}

type projectPreviewRequest struct {
	path   string
	cancel context.CancelFunc
}

func (d *Desktop) cancelProjectPreview() {
	if d.projectLoad != nil {
		d.projectLoad.cancel()
		d.projectLoad = nil
	}
}

func (d *Desktop) resetProjectFiles() {
	d.rememberProjectTab()
	d.cancelProjectPreview()
	d.projectFiles, d.projectTree, d.projectFileRow = nil, nil, -1
	d.projectShownPath, d.projectLoadError, d.projectFailedPath = "", "", ""
	d.projectPreview, d.projectBitmap, d.projectCode = app.FilePreview{}, nil, nil
	d.projectLargeText = false
}

func (d *Desktop) loadProjectPreview(path string) {
	if d.Client == nil {
		return
	}
	d.cancelProjectPreview()
	params := ipc.Params{RepoID: d.selected, Path: path}
	shownVersion := ""
	if d.projectShownPath == path {
		shownVersion = d.projectPreview.Version
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	request := &projectPreviewRequest{path: path, cancel: cancel}
	d.projectLoad, d.projectLoadError, d.projectFailedPath = request, "", ""
	go func() {
		defer cancel()
		preview, bitmap, code, err := fetchProjectPreview(ctx, d.Client, params, shownVersion)
		mygo.RunOnMain(func() {
			if d.projectLoad != request || d.page != projectFilesPage || d.win != nil && d.win.IsDestroyed() {
				return
			}
			d.projectLoad = nil
			if err != nil {
				d.projectLoadError, d.projectFailedPath = err.Error(), path
			} else if d.projectShownPath != path || preview.Version == "" || preview.Version != d.projectPreview.Version {
				d.rememberProjectTab()
				d.projectShownPath = path
				d.projectPreview, d.projectBitmap = preview, bitmap
				d.projectCode, d.projectLargeText = code, preview.NextOffset != 0
				d.projectCodePath, d.projectCodeSource = path, preview.New
				d.projectFormatted, d.projectFormatError = false, ""
				if d.findProjectTab(path) == nil {
					if d.projectTabs == nil {
						d.projectTabs = make(map[string][]*projectTab)
					}
					d.projectTabs[d.selected] = append(d.projectTabs[d.selected], &projectTab{path: path})
				}
				d.rememberProjectTab()
			}
			if d.win != nil {
				d.win.Invalidate()
			}
		})
	}()
}

func fetchProjectPreview(ctx context.Context, client *ipc.Client, params ipc.Params, shownVersion string) (app.FilePreview, *native.Bitmap, *editor.State, error) {
	var preview app.FilePreview
	if err := client.Call(ctx, "git.file.preview", params, &preview); err != nil {
		return app.FilePreview{}, nil, nil, err
	}
	if shownVersion != "" && preview.Version == shownVersion {
		return preview, nil, nil, nil
	}
	var bitmap *native.Bitmap
	var code *editor.State
	if len(preview.ImagePNG) != 0 {
		decoded, err := png.Decode(bytes.NewReader(preview.ImagePNG))
		if err != nil {
			return app.FilePreview{}, nil, nil, err
		}
		bitmap = native.NewBitmap(decoded)
		preview.ImagePNG = nil
	} else if preview.NextOffset != 0 {
		var err error
		code, err = readProjectText(ctx, client, params, preview)
		if err != nil {
			return app.FilePreview{}, nil, nil, err
		}
		preview.New = ""
	} else if !preview.Binary {
		code = codePreview(params.Path, preview.New)
	}
	return preview, bitmap, code, nil
}

// IPC still transfers bounded chunks. The editor receives one document and
// lays out only the visible lines as the user scrolls.
func readProjectText(ctx context.Context, client *ipc.Client, params ipc.Params, first app.FilePreview) (*editor.State, error) {
	var text strings.Builder
	text.WriteString(first.New)
	params.PreviewVersion = first.Version
	for next := first.NextOffset; next != 0; {
		params.PreviewOffset = next
		var part app.FilePreview
		if err := client.Call(ctx, "git.file.preview", params, &part); err != nil {
			return nil, err
		}
		if part.Binary || part.Version != first.Version || part.NextOffset != 0 && part.NextOffset <= next {
			return nil, fmt.Errorf("file could not be read as one text document")
		}
		text.WriteString(part.New)
		next = part.NextOffset
	}
	return codePreview(params.Path, text.String()), nil
}

func projectFileIcon(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".mdx", ".rst", ".txt", ".pdf", ".rtf", ".doc", ".docx":
		return "changes"
	case ".json", ".jsonc", ".yaml", ".yml", ".toml", ".xml", ".ini", ".csv", ".tsv", ".env", ".lock", ".mod", ".sum":
		return "file-data"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp", ".avif", ".tiff":
		return "file-image"
	case ".mp3", ".wav", ".ogg", ".flac", ".mp4", ".mov", ".webm", ".mkv":
		return "file-media"
	case ".zip", ".gz", ".tar", ".tgz", ".xz", ".7z", ".rar":
		return "file-archive"
	}
	switch strings.ToLower(name) {
	case "readme", "license", "copying", "changelog":
		return "changes"
	case ".gitignore", ".gitattributes", ".editorconfig", ".gitmodules":
		return "file-data"
	}
	if editor.LanguageFor(name) != nil {
		return "file-code"
	}
	return "file"
}

func codePreview(filename, source string) *editor.State {
	buffer := editor.NewBuffer(source)
	buffer.SetLanguage(editor.LanguageForText(filename, source))
	return &editor.State{Buffer: buffer, ReadOnly: true, NoScrollPastEnd: true, KeepScrollbars: true}
}

func (d *Desktop) showCodePreview(c *native.Context, filename string) {
	source := d.projectPreview.New
	if d.projectCode == nil || d.projectCodePath != filename {
		d.projectCode = codePreview(filename, source)
		d.projectCodePath, d.projectCodeSource = filename, source
		d.projectFormatted, d.projectFormatError = false, ""
	}
	native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Wrap().Children(func() {
		if d.projectEditable() {
			if IconButton(c, d.copy("保存", "Save"), "save", false).Disabled(d.busy || !d.projectDirty()).Clicked() {
				d.saveProjectTab()
			}
			if !d.busy && d.projectDirty() && !d.settingsOpen && !d.repositoryTools.confirmOpen && c.Shortcut(native.Cmd, native.KeyS) {
				d.saveProjectTab()
			}
		} else if d.projectPreview.Version != "" {
			native.Text(c, d.copy("只读 · 超过 5 MiB", "Read-only · Over 5 MiB")).TextColor(c.Theme().TextMuted)
		}
		d.projectFileActions(c, filename)
		native.Checkbox(c, &d.projectWrap, d.copy("自动换行", "Wrap lines"))
		if lang := d.projectCode.Buffer.Language(); lang != nil {
			native.Text(c, lang.Name).TextColor(c.Theme().TextMuted)
		}
		ext := strings.ToLower(path.Ext(filename))
		if d.projectLargeText || d.projectDirty() || ext != ".go" && ext != ".json" {
			return
		}
		label := d.copy("格式化预览", "Format preview")
		if d.projectFormatted {
			label = d.copy("显示原文", "Show original")
		}
		if !native.Button(c, label).Clicked() {
			return
		}
		d.projectFormatError = ""
		if d.projectFormatted {
			d.projectCode.Buffer.Reset(source)
			d.projectFormatted = false
			return
		}
		var formatted []byte
		var err error
		if ext == ".go" {
			formatted, err = format.Source([]byte(source))
		} else {
			var out bytes.Buffer
			err = json.Indent(&out, []byte(source), "", "  ")
			formatted = out.Bytes()
		}
		if err != nil {
			d.projectFormatError = err.Error()
			return
		}
		d.projectCode.Buffer.Reset(string(formatted))
		d.projectFormatted = true
	})
	if d.projectFormatError != "" {
		native.Text(c, d.copy("格式化失败", "Formatting failed")+" · "+d.projectFormatError).FillWidth().TextColor(c.Theme().Danger)
	}
	d.projectCode.SoftWrap = d.projectWrap
	d.projectCode.ReadOnly = d.busy || d.projectFormatted || !d.projectEditable()
	d.projectCode.Font.Size = c.Theme().FontSize - 0.5
	editor.View(c, d.projectCode).Grow(1).FillWidth()
}
