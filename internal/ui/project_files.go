package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"image/png"
	"path"
	"strings"

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
}

func (d *Desktop) showProjectFiles(c *native.Context, t *native.Theme) {
	native.Row(c).Grow(1).FillWidth().Gap(panelSpacing).Children(func() {
		native.Column(c).Width(342).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			before := d.projectSearch
			native.TextInput(c, &d.projectSearch).Placeholder(d.text("files.search")).FillWidth().Disabled(d.busy)
			if before != d.projectSearch {
				d.projectFileRow, d.projectPreview = -1, app.FilePreview{}
				d.projectBitmap = nil
				d.projectLargeText = false
				d.projectCode = nil
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
					if d.projectCollapsed == nil {
						d.projectCollapsed = make(map[string]bool)
					}
					d.projectCollapsed[entry.path] = !d.projectCollapsed[entry.path]
					d.projectFileRow, d.projectPreview = -1, app.FilePreview{}
					d.projectBitmap = nil
					d.projectLargeText = false
					d.projectCode = nil
					d.rebuildProjectTree()
					return
				}
				d.projectPreview = app.FilePreview{}
				d.projectBitmap = nil
				d.projectCode = nil
				d.projectLargeText = false
				d.loadProjectPreview(entry.path)
			}
		})
		native.Column(c).Grow(1).FillHeight().Padding(panelSpacing).Gap(controlSpacing).Background(t.Background).Radius(8).Children(func() {
			if row := d.projectFileRow; row < 0 || row >= len(d.projectTree) {
				native.Text(c, d.text("files.select")).TextColor(t.TextMuted)
			} else {
				filename := d.projectTree[row].path
				native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
					native.Text(c, filename).Bold().Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Tooltip(filename)
					if d.err != "" && IconButton(c, d.text("refresh"), "refresh", false).Disabled(d.busy).Clicked() {
						d.loadProjectPreview(filename)
					}
					if IconButton(c, d.text("file.history"), "history", false).Disabled(d.busy).Clicked() {
						d.openFileHistory(filename)
					}
				})
				if d.projectBitmap != nil {
					native.Image(c, d.projectBitmap).Grow(1).MinHeight(0).FillWidth().Fit(native.Contain).Label(filename)
				} else if d.projectPreview.Binary {
					native.Text(c, d.text("diff.binary")).TextColor(t.TextMuted)
				} else if !d.busy && d.err == "" {
					d.showCodePreview(c, filename)
				}
			}
		})
	})
}

func (d *Desktop) loadProjectPreview(path string) {
	params := ipc.Params{RepoID: d.selected, Path: path}
	d.task(func(ctx context.Context) (func(), error) {
		var preview app.FilePreview
		if err := d.Client.Call(ctx, "git.file.preview", params, &preview); err != nil {
			return nil, err
		}
		var bitmap *native.Bitmap
		var code *editor.State
		if len(preview.ImagePNG) != 0 {
			decoded, err := png.Decode(bytes.NewReader(preview.ImagePNG))
			if err != nil {
				return nil, err
			}
			bitmap = native.NewBitmap(decoded)
			preview.ImagePNG = nil
		} else if preview.NextOffset != 0 {
			var err error
			code, err = readProjectText(ctx, d.Client, params, preview)
			if err != nil {
				return nil, err
			}
			preview = app.FilePreview{}
		}
		return func() {
			d.projectPreview, d.projectBitmap = preview, bitmap
			d.projectCode, d.projectLargeText = code, code != nil
			d.projectCodePath, d.projectCodeSource = path, ""
			d.projectFormatted, d.projectFormatError = false, ""
		}, nil
	})
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
	editor.View(c, d.projectCode).Grow(1).FillWidth()
}
