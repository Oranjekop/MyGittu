package main

import "github.com/ebitengine/purego/objc"

func configureRemoteCaption(handle uintptr) {
	window := objc.ID(handle)
	standardButton := objc.RegisterName("standardWindowButton:")
	setHidden := objc.RegisterName("setHidden:")
	const (
		miniaturizeButton = 1
		zoomButton        = 2
	)
	window.Send(standardButton, miniaturizeButton).Send(setHidden, true)
	window.Send(standardButton, zoomButton).Send(setHidden, true)
}
