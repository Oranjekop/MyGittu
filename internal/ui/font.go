package ui

import (
	_ "embed"
	"log"
	"sync"

	native "github.com/egoist/mygo/ui"
)

//go:embed oppo-sans.ttf
var oppoSans []byte

const appFontName = "OPPO Sans 4.0"

var appFontFamily = sync.OnceValue(func() string {
	if err := native.RegisterFont(oppoSans, appFontName); err != nil {
		log.Printf("MyGittu: register OPPO Sans: %v", err)
		return ""
	}
	return appFontName
})
