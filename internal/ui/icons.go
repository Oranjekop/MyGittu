package ui

import (
	native "github.com/egoist/mygo/ui"
	"strings"
	"time"
)

// Original 20px line icons: one stroke weight and round ends throughout.
// SVGs are parsed once; rendering follows the current theme and scale factor.
var iconShapes = map[string]string{
	"close":        `<path d="m5 5 10 10M15 5 5 15"/>`,
	"clear":        `<rect x="3" y="3" width="14" height="14" rx="3"/><path d="m7.5 7.5 5 5m0-5-5 5"/>`,
	"plus":         `<path d="M10 4v12M4 10h12"/>`,
	"minus":        `<path d="M4 10h12"/>`,
	"left":         `<path d="m12 4-6 6 6 6"/>`,
	"right":        `<path d="m8 4 6 6-6 6"/>`,
	"sparkle":      `<path d="M9 3c1 4 2 5 6 6-4 1-5 2-6 6-1-4-2-5-6-6 4-1 5-2 6-6Zm6 10v5m-2.5-2.5h5"/>`,
	"prompt":       `<rect x="3" y="4" width="14" height="12" rx="2"/><path d="m6 8 2 2-2 2m4 0h4"/>`,
	"save":         `<path d="M4 3h10l3 3v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Zm2 0v5h7V3M6 17v-5h8v5"/>`,
	"download":     `<path d="M10 2v10m-4-4 4 4 4-4M3 13v3a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-3"/>`,
	"appearance":   `<rect x="3" y="3" width="14" height="11" rx="2"/><path d="M10 14v3m-3 0h6M6 8h8"/>`,
	"conflict":     `<path d="M9 3.5a1.2 1.2 0 0 1 2 0l6.5 11.2a1.2 1.2 0 0 1-1 1.8h-13a1.2 1.2 0 0 1-1-1.8ZM10 7v4m0 2.5v.1"/>`,
	"undo":         `<path d="M7 3 3 7l4 4M3 7h9a4.5 4.5 0 0 1 0 9H9"/>`,
	"revert":       `<circle cx="12.5" cy="12.5" r="3.5"/><path d="M9 12.5H7a3.5 3.5 0 0 1 0-7h5m-3-3 3 3-3 3m7 4h2"/>`,
	"restore":      `<path d="M3 11v5h14v-5M10 12V3M6.5 6.5 10 3l3.5 3.5"/>`,
	"tag":          `<path d="M3 4v6l6.5 6.5a1.5 1.5 0 0 0 2.1 0l4.9-4.9a1.5 1.5 0 0 0 0-2.1L10 3H4a1 1 0 0 0-1 1Zm4 3h.1"/>`,
	"switch":       `<path d="M3 7h14m-3-3 3 3-3 3m3 3H3m3-3-3 3 3 3"/>`,
	"merge":        `<circle cx="6" cy="4" r="2"/><circle cx="6" cy="16" r="2"/><circle cx="15" cy="4" r="2"/><path d="M6 6v8m9-8a6 6 0 0 1-6 6H6"/>`,
	"stash":        `<path d="M3 5h14v4H3zM4 9v7h12V9M8 12h4"/>`,
	"open":         `<path d="M2.5 6.5V5a1.5 1.5 0 0 1 1.5-1.5h4l2 2h6A1.5 1.5 0 0 1 17.5 7v1M2.5 8.5h15l-2 8h-12z"/>`,
	"clone":        `<rect x="7" y="7" width="10" height="10" rx="2"/><path d="M12 3H5a2 2 0 0 0-2 2v7M9.5 12h5M12 9.5v5"/>`,
	"changes":      `<path d="M11 3H5a1.5 1.5 0 0 0-1.5 1.5v11A1.5 1.5 0 0 0 5 17h10a1.5 1.5 0 0 0 1.5-1.5V8zM11 3v5h5.5M6.5 11h5M6.5 14h3"/>`,
	"history":      `<path d="M3 9a7 7 0 1 1 1.6 5.6M3 4v5h5M10 6v4l3 2"/>`,
	"branch":       `<circle cx="6" cy="4" r="2"/><circle cx="6" cy="16" r="2"/><circle cx="15" cy="5" r="2"/><path d="M6 6v8M15 7v1a4 4 0 0 1-4 4H6"/>`,
	"settings":     `<path d="M3 5h14M3 10h14M3 15h14"/><rect x="6" y="3" width="3" height="4" rx="1"/><rect x="12" y="8" width="3" height="4" rx="1"/><rect x="6" y="13" width="3" height="4" rx="1"/>`,
	"fetch":        `<path d="M5 13H4a3 3 0 0 1-.2-6A5.5 5.5 0 0 1 14.4 6a3.5 3.5 0 1 1 1.1 7H15M10 9v8M7 14l3 3 3-3"/>`,
	"pull":         `<path d="M5 3v12M2 12l3 3 3-3M15 3v14M12 14l3 3 3-3"/>`,
	"push":         `<path d="M3 13v2a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-2M10 13V3M6 7l4-4 4 4"/>`,
	"refresh":      `<path d="M16.5 8A6.7 6.7 0 0 0 5 5L2.5 7.5M2.5 3v4.5H7M3.5 12A6.7 6.7 0 0 0 15 15l2.5-2.5M17.5 17v-4.5H13"/>`,
	"stage":        `<path d="M3 11v6h14v-6M3 11h4l1.5 2h3L13 11h4M10 2v6M7 5h6"/>`,
	"unstage":      `<path d="M3 11v6h14v-6M3 11h4l1.5 2h3L13 11h4M7 5h6"/>`,
	"commit":       `<circle cx="10" cy="10" r="4"/><path d="M2 10h4M14 10h4M8.3 10l1.1 1.2 2.4-2.5"/>`,
	"search":       `<circle cx="8.5" cy="8.5" r="5.5"/><path d="m12.5 12.5 4.5 4.5"/>`,
	"user":         `<circle cx="10" cy="6" r="3"/><path d="M4 17v-2a6 6 0 0 1 12 0v2"/>`,
	"down":         `<path d="m4 7 6 6 6-6"/>`,
	"folder":       `<path d="M2.5 6V5a1.5 1.5 0 0 1 1.5-1.5h4l2 2h6a1.5 1.5 0 0 1 1.5 1.5v8a1.5 1.5 0 0 1-1.5 1.5H4a1.5 1.5 0 0 1-1.5-1.5z"/>`,
	"file-code":    `<path d="m7 5-5 5 5 5m6-10 5 5-5 5m-2-12-2 14"/>`,
	"file-data":    `<path d="M7 3H5a2 2 0 0 0-2 2v3l-1 2 1 2v3a2 2 0 0 0 2 2h2m6-14h2a2 2 0 0 1 2 2v3l1 2-1 2v3a2 2 0 0 1-2 2h-2M7 10h.1m2.9 0h.1m2.9 0h.1"/>`,
	"file-image":   `<rect x="2.5" y="3.5" width="15" height="13" rx="2"/><circle cx="7" cy="8" r="1.25"/><path d="m3 14 4-3 3 2 3.5-4 3.5 4"/>`,
	"file-media":   `<rect x="2.5" y="4" width="15" height="12" rx="2"/><path d="m8 7 5 3-5 3z"/>`,
	"file-archive": `<rect x="4" y="2.5" width="12" height="15" rx="2"/><path d="M9 3v2h2v2H9v2h2v2H9"/><rect x="8" y="12" width="4" height="3" rx="1"/>`,
	"file":         `<path d="M11 2.5H5a1.5 1.5 0 0 0-1.5 1.5v12A1.5 1.5 0 0 0 5 17.5h10a1.5 1.5 0 0 0 1.5-1.5V8zM11 2.5V8h5.5"/>`,
	"remove":       `<path d="M4 6h12M8 3h4M5.5 6l1 11h7l1-11M8.5 9v5M11.5 9v5"/>`,
	"diff":         `<rect x="2.5" y="3" width="15" height="14" rx="2"/><path d="M10 3v14M5 8h2.5M12.5 10H15M13.75 8.75v2.5"/>`,
}

var icons = func() map[string]*native.SVG {
	result := make(map[string]*native.SVG, len(iconShapes))
	for name, shape := range iconShapes {
		result[name] = native.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round">` + shape + `</svg>`))
	}
	return result
}()

// IconButton renders an action with an accessible text label.
func IconButton(c *native.Context, label, icon string, primary bool) native.Element {
	var button native.Element
	if primary {
		button = native.PrimaryButton(c, "")
	} else {
		button = native.Button(c, "")
	}
	return button.Label(label).Height(controlHeight(c)).Padding(0, 8).Gap(6).AlignItems(native.Center).Children(func() {
		if vector := icons[strings.TrimPrefix(icon, "remote.")]; vector != nil {
			native.Icon(c, vector).Width(14).Height(14)
		}
		if label != "" {
			native.Text(c, label).SingleLine()
		}
	})
}

func controlHeight(c *native.Context) float32 { return max(27, c.Theme().FontSize+14.5) }

func (d *Desktop) refreshButton(c *native.Context) native.Element {
	label := d.text("refresh")
	if d.busy {
		label = d.copy("正在刷新", "Refreshing")
	}
	return native.Button(c, "").Label(label).Tooltip(label).Height(controlHeight(c)).Padding(0, 8).Children(func() {
		icon := native.Icon(c, icons["refresh"]).Width(14).Height(14)
		if d.busy {
			icon.Rotate(icon.Loop("busy-refresh", 900*time.Millisecond, native.Linear) * 360)
		}
	})
}
