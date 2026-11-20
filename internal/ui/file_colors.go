package ui

import (
	native "github.com/egoist/mygo/ui"
	git "mygittu/internal/git"
)

type filePaletteColor struct {
	onLight, onDark native.Color
}

func (color filePaletteColor) foreground(darkBackground bool) native.Color {
	if darkBackground {
		return color.onDark
	}
	return color.onLight
}

var (
	fileBlue   = filePaletteColor{native.Hex("#215eaa"), native.Hex("#9bc4ff")}
	fileGreen  = filePaletteColor{native.Hex("#1b6b42"), native.Hex("#8bd9a8")}
	filePurple = filePaletteColor{native.Hex("#69449d"), native.Hex("#cbb0ff")}
	fileAmber  = filePaletteColor{native.Hex("#855200"), native.Hex("#f3c578")}
	fileRed    = filePaletteColor{native.Hex("#a33338"), native.Hex("#ffaaaa")}
	fileTeal   = filePaletteColor{native.Hex("#166a72"), native.Hex("#8edbe2")}
	fileGray   = filePaletteColor{native.Hex("#565c65"), native.Hex("#bac3cf")}
)

var fileIconColors = map[string]filePaletteColor{
	"file-code": fileBlue, "file-data": fileAmber, "changes": fileTeal,
	"file-image": fileGreen, "file-media": filePurple, "file-archive": fileRed,
	"folder": fileAmber, "open": fileAmber,
}

func fileIconColor(icon string, darkBackground bool) native.Color {
	if color, ok := fileIconColors[icon]; ok {
		return color.foreground(darkBackground)
	}
	return fileGray.foreground(darkBackground)
}

var changeStatusColors = map[git.State]filePaletteColor{
	git.Modified: fileBlue, git.Added: fileGreen, git.Deleted: fileRed,
	git.Untracked: filePurple, git.Conflicted: fileAmber,
}

func changeStatusColor(state git.State, darkBackground bool) native.Color {
	if color, ok := changeStatusColors[state]; ok {
		return color.foreground(darkBackground)
	}
	return fileGray.foreground(darkBackground)
}
