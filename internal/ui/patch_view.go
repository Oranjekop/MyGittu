package ui

import (
	"strconv"
	"strings"

	native "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type patchLine struct {
	old, next int
	kind      byte
	text      string
}

type patchView struct {
	list  native.ListState
	lines []patchLine
}

func (d *Desktop) showCodeDiff(c *native.Context, diff *editor.DiffState, width float32, fullContext bool) {
	t := c.Theme()
	native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
		native.Segmented(c, &d.diffLayout, d.copy("自动", "Auto"), d.copy("并排", "Side by side"), d.copy("统一", "Unified"))
		native.Checkbox(c, &d.diffWrap, d.copy("自动换行", "Wrap lines"))
		if fullContext {
			native.Checkbox(c, &d.diffExpand, d.copy("展开上下文", "Expand context"))
		}
	})
	diff.Font.Size = t.FontSize - 0.5
	diff.Layout = editor.DiffSplit
	if d.diffLayout == 2 || d.diffLayout == 0 && width < 550 {
		diff.Layout = editor.DiffUnified
	}
	diff.Wrap = d.diffWrap
	diff.ExpandUnchanged = fullContext && d.diffExpand
	diff.KeepScrollbars = true
	if diff.Layout == editor.DiffSplit {
		// MyGo uses the accent for the divider while hovered or dragged.
		diffTheme := *t
		diffTheme.Accent = t.Border
		native.Themed(c, &diffTheme).Grow(1).FillWidth().Children(func() {
			editor.DiffView(c, diff).Grow(1).FillWidth()
		})
	} else {
		editor.DiffView(c, diff).Grow(1).FillWidth()
	}
}

func (d *Desktop) patchRows(files []editor.FilePatch, fileHeaders bool) []patchLine {
	var rows []patchLine
	for _, file := range files {
		if fileHeaders {
			rows = append(rows, patchLine{kind: 'h', text: patchFileName(file)})
		}
		if file.Binary {
			rows = append(rows, patchLine{text: d.text("diff.binary")})
		} else if len(file.Hunks) == 0 {
			rows = append(rows, patchLine{text: d.copy("文件属性或路径变更", "File properties or path changed")})
		}
		for _, h := range file.Hunks {
			if h.Section != "" {
				rows = append(rows, patchLine{kind: 'h', text: h.Section})
			}
			old, next := h.OldStart, h.NewStart
			for _, line := range h.Lines {
				if line == "" {
					continue
				}
				row := patchLine{kind: line[0], text: strings.TrimSuffix(line[1:], "\r")}
				switch row.kind {
				case '+':
					row.next = next
					next++
				case '-':
					row.old = old
					old++
				case ' ':
					row.old, row.next = old, next
					old++
					next++
				case '\\':
					row.text = d.copy("文件末尾无换行", "No newline at end of file")
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func patchFileName(file editor.FilePatch) string {
	if file.NewName == "" {
		return file.OldName
	}
	if file.OldName != "" && file.OldName != file.NewName {
		return file.OldName + " → " + file.NewName
	}
	return file.NewName
}

func (d *Desktop) showPatch(c *native.Context, view *patchView) native.Element {
	t := c.Theme()
	return roundedTable(c, &view.list, []native.TableColumn{
		{Title: d.copy("旧行", "Old"), Width: 54},
		{Title: d.copy("新行", "New"), Width: 54},
		{Title: d.copy("代码变更", "Code changes")},
	}, len(view.lines), func(row, col int) {
		line := view.lines[row]
		number := line.old
		if col == 1 {
			number = line.next
		}
		if col < 2 {
			if number > 0 {
				native.Text(c, strconv.Itoa(number)).TextColor(t.TextMuted)
			}
			return
		}
		text, color := line.text, t.Text
		switch line.kind {
		case '+':
			text, color = "+  "+text, t.Success
		case '-':
			text, color = "−  "+text, t.Danger
		case 'h':
			native.Text(c, text).Bold().SingleLine().Tooltip(text).TextColor(t.TextMuted)
			return
		default:
			text = "   " + text
		}
		native.Text(c, strings.ReplaceAll(text, "\t", "    ")).Font("monospace").SingleLine().Ellipsis("…").Selectable().Tooltip(line.text).TextColor(color)
	}).FillWidth()
}
