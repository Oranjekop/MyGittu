package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"image/png"
	"path"
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
	seen := make(map[string]bool)
	for _, file := range d.projectFiles {
		if !strings.Contains(strings.ToLower(file), query) {
			continue
		}
		parts := strings.Split(file, "/")
		var dir string
		hidden := false
		for _, part := range parts[:len(parts)-1] {
			dir = path.Join(dir, part)
			if !seen[dir] {
				d.projectTree = append(d.projectTree, projectEntry{dir, true})
				seen[dir] = true
			}
			if query == "" && d.projectCollapsed[dir] {
				hidden = true
				break
			}
		}
		if !hidden {
			d.projectTree = append(d.projectTree, projectEntry{path: file})
		}
	}
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
			if len(d.projectFiles) == 0 && !d.busy {
				native.Text(c, d.text("files.empty")).TextColor(t.TextMuted)
			}
			d.projectFileList.Selected = &d.projectFileRow
			previous := d.projectFileRow
			native.Table(c, &d.projectFileList, []native.TableColumn{{Title: d.text("nav.files")}}, len(d.projectTree), func(row, _ int) {
				entry := d.projectTree[row]
				name := path.Base(entry.path)
				native.Row(c).FillWidth().MinWidth(0).Gap(detailSpacing).AlignItems(native.Center).
					PaddingStart(float32(strings.Count(entry.path, "/") * 12)).Tooltip(entry.path).Children(func() {
					icon := "open"
					if entry.dir {
						arrow := "down"
						if d.projectSearch == "" && d.projectCollapsed[entry.path] {
							icon, arrow = "folder", "right"
						}
						native.Icon(c, icons[arrow]).Width(12).Height(12)
					} else {
						native.Box(c).Width(12)
						icon = projectFileIcon(name)
					}
					native.Icon(c, icons[icon]).Width(15).Height(15)
					native.Text(c, name).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				})
			}).Grow(1).FillWidth().Disabled(d.busy)
			if row := d.projectFileRow; row != previous && row >= 0 && row < len(d.projectTree) && !d.busy {
				entry := d.projectTree[row]
				if entry.dir {
					d.cancelProjectPreview()
					if d.projectCollapsed == nil {
						d.projectCollapsed = make(map[string]bool)
					}
					d.projectCollapsed[entry.path] = !d.projectCollapsed[entry.path]
					d.rebuildProjectTree()
					return
				}
				d.loadProjectPreview(entry.path)
			}
		})
		native.Column(c).Grow(1).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			filename := d.projectShownPath
			if filename == "" && d.projectLoad == nil {
				filename = d.selectedProjectPath()
			}
			if filename == "" && d.projectLoad != nil {
				native.Text(c, d.text("busy")).TextColor(t.TextMuted)
			} else if filename == "" {
				native.Text(c, d.text("files.select")).TextColor(t.TextMuted)
			} else {
				native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
					native.Text(c, filename).Bold().Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(filename)
					if failed := d.selectedProjectPath(); d.projectLoadError != "" && failed != "" && IconButton(c, d.text("refresh"), "refresh", false).Disabled(d.busy).Clicked() {
						d.loadProjectPreview(failed)
					}
					if IconButton(c, d.text("file.history"), "history", false).Disabled(d.busy).Clicked() {
						d.openFileHistory(filename)
					}
				})
				if d.projectBitmap != nil {
					native.Image(c, d.projectBitmap).Grow(1).MinHeight(0).FillWidth().Fit(native.Contain).Label(filename)
				} else if d.projectPreview.Binary {
					native.Text(c, d.text("diff.binary")).TextColor(t.TextMuted)
				} else if d.projectShownPath != "" || d.projectLoadError == "" {
					d.showCodePreview(c, filename)
				}
			}
			if d.projectLoadError != "" {
				native.Text(c, d.projectLoadError).FillWidth().TextColor(t.Danger)
			}
		})
	})
}

type projectPreviewRequest struct {
	cancel context.CancelFunc
}

func (d *Desktop) cancelProjectPreview() {
	if d.projectLoad != nil {
		d.projectLoad.cancel()
		d.projectLoad = nil
	}
}

func (d *Desktop) resetProjectFiles() {
	d.cancelProjectPreview()
	d.projectFiles, d.projectTree, d.projectFileRow = nil, nil, -1
	d.projectShownPath, d.projectLoadError = "", ""
	d.projectPreview, d.projectBitmap, d.projectCode = app.FilePreview{}, nil, nil
	d.projectLargeText = false
	d.projectScroll = projectWheelScroll{}
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
	request := &projectPreviewRequest{cancel: cancel}
	d.projectLoad, d.projectLoadError = request, ""
	go func() {
		defer cancel()
		preview, bitmap, code, err := fetchProjectPreview(ctx, d.Client, params, shownVersion)
		mygo.RunOnMain(func() {
			if d.projectLoad != request || d.page != projectFilesPage || d.win != nil && d.win.IsDestroyed() {
				return
			}
			d.projectLoad = nil
			if err != nil {
				d.projectLoadError = err.Error()
			} else if d.projectShownPath != path || preview.Version == "" || preview.Version != d.projectPreview.Version {
				d.projectShownPath = path
				d.projectPreview, d.projectBitmap = preview, bitmap
				d.projectCode, d.projectLargeText = code, preview.NextOffset != 0
				d.projectScroll = projectWheelScroll{}
				d.projectCodePath, d.projectCodeSource = path, preview.New
				d.projectFormatted, d.projectFormatError = false, ""
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
	if d.projectCode == nil || d.projectCodePath != filename || d.projectCodeSource != source {
		d.projectCode = codePreview(filename, source)
		d.projectCodePath, d.projectCodeSource = filename, source
		d.projectFormatted, d.projectFormatError = false, ""
	}
	native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
		native.Checkbox(c, &d.projectWrap, d.copy("自动换行", "Wrap lines"))
		if lang := d.projectCode.Buffer.Language(); lang != nil {
			native.Text(c, lang.Name).TextColor(c.Theme().TextMuted)
		}
		ext := strings.ToLower(path.Ext(filename))
		if d.projectLargeText || ext != ".go" && ext != ".json" {
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
	d.projectCode.Font.Size = c.Theme().FontSize - 0.5
	d.projectScroll.update(c, d.projectCode)
	editor.View(c, d.projectCode).Grow(1).FillWidth()
}

type projectWheelScroll struct {
	code         *editor.State
	start        time.Time
	from, target float64
	reduced      bool
}

func (s *projectWheelScroll) update(c *native.Context, code *editor.State) {
	const duration = 140 * time.Millisecond
	if s.code != code {
		*s = projectWheelScroll{code: code}
		services := c.Services()
		code.OnInput = func(ev native.InputEvent) bool { return s.input(ev, code, services) }
	}
	s.reduced = c.Preferences().ReduceMotion
	if !s.start.IsZero() {
		progress := min(1, max(0, float64(c.Now().Sub(s.start))/float64(duration)))
		if s.reduced || progress >= 1 {
			code.SetScrollY(s.target)
			s.start = time.Time{}
		} else {
			remaining := 1 - progress
			code.SetScrollY(s.target - (s.target-s.from)*remaining*remaining*remaining)
			c.After(time.Second / 60)
		}
	}
}

func (s *projectWheelScroll) input(ev native.InputEvent, code *editor.State, services native.Services) bool {
	if s.code != code {
		return false
	}
	if ev.Kind == native.InputPointerDown || ev.Kind == native.InputKeyDown {
		s.start = time.Time{}
	}
	if ev.Kind != native.InputScroll {
		return false
	}
	if s.reduced || ev.Precise || ev.DX != 0 || ev.DY == 0 || ev.Mods != 0 {
		s.start = time.Time{}
		return false
	}
	current := code.ScrollY()
	target := current
	if !s.start.IsZero() {
		target = s.target
	}
	limit := max(0, code.LineTop(code.Buffer.LineCount())-code.ViewportHeight())
	target = min(limit, max(0, target+float64(ev.DY)))
	if target == current && s.start.IsZero() {
		return false
	}
	s.from, s.target, s.start = current, target, ev.Time
	services.Invalidate()
	return true
}
