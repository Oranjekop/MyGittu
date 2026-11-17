package main

// Regenerate executable resources after changing app.ico.
//go:generate go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -ico app.ico -o resource_windows_amd64.syso
//go:generate go run github.com/akavel/rsrc@v0.10.2 -arch arm64 -ico app.ico -o resource_windows_arm64.syso
