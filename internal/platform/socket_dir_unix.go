//go:build !windows && !darwin

package platform

func socketDir() (string, error) {
	return ConfigDir()
}
