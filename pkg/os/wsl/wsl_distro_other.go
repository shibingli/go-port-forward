//go:build !windows

package wsl

// ListDistros returns all installed WSL distributions.
// On non-Windows platforms, it always returns an error.
func ListDistros() ([]Distro, error) {
	return nil, ErrNotSupported
}

// GetIP returns the IP address of a WSL2 distribution.
// On non-Windows platforms, it always returns an error.
func GetIP(distro string) (string, error) {
	return "", ErrNotSupported
}

// ListPorts returns TCP and UDP ports that are listening inside a WSL2 distro.
// On non-Windows platforms, it always returns an error.
func ListPorts(distro string) ([]Port, error) {
	return nil, ErrNotSupported
}
