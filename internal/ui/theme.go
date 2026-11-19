package ui

import native "github.com/egoist/mygo/ui"

const (
	panelSpacing   = 10
	controlSpacing = 8
	detailSpacing  = 4
	sidebarWidth   = 36
)

func ApplyTheme(c *native.Context, fontSize float32) *native.Theme {
	t := *c.Theme()
	t.Font = appFontFamily()
	t.FontSize = fontSize
	t.Spacing = 3.5
	if t.Dark {
		t.Background = native.Hex("#242424")
		t.Surface = native.Hex("#191919")
		t.SurfaceHover = native.Hex("#333333")
		t.SurfacePressed = native.Hex("#404040")
		t.Border = native.Hex("#424242")
		t.Accent = native.Hex("#d9d9d9")
		t.AccentText = native.Hex("#171717")
		t.AccentHover = native.Hex("#ededed")
		t.AccentPressed = native.Hex("#bdbdbd")
	} else {
		t.Background = native.Hex("#ffffff")
		t.Surface = native.Hex("#f1f1f1")
		t.SurfaceHover = native.Hex("#e6e6e6")
		t.SurfacePressed = native.Hex("#dcdcdc")
		t.Border = native.Hex("#d4d4d4")
		t.Accent = native.Hex("#292929")
		t.AccentText = native.Hex("#ffffff")
		t.AccentHover = native.Hex("#414141")
		t.AccentPressed = native.Hex("#161616")
	}
	t.InactiveAccent = t.Accent
	c.SetTheme(&t)
	return &t
}

func roundedTable(c *native.Context, state *native.ListState, columns []native.TableColumn, rows int, cell func(row, column int)) native.Element {
	state.RoundRows = true
	return native.Table(c, state, columns, rows, cell)
}
