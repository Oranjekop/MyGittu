//go:build !windows && !darwin

package ui

import (
	"context"
	"errors"
)

const explorerSupported = false

func modernSupported() bool { return false }
func readIntegration(context.Context) (integrationState, error) {
	return integrationState{}, errors.New("Login and Explorer integration are unavailable on this platform.")
}
func changeIntegration(context.Context, string, bool) error {
	return errors.New("Integration is unavailable on this platform.")
}
