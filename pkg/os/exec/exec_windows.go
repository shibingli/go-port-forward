//go:build windows

package exec

import (
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Windows process creation flags:
// CREATE_NO_WINDOW (0x08000000) — prevents console window from appearing
// CREATE_NEW_PROCESS_GROUP (0x00000200) — allows clean process group management
var SysProcAttr = &syscall.SysProcAttr{
	HideWindow:    true,
	CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
}

// init ensures the current process has a complete PATH that includes
// system and user PATH entries from the registry. This is critical when
// running as a Windows service, where the inherited environment may be
// minimal and miss entries like C:\Windows\System32 (for wsl.exe, etc.).
func init() {
	mergeRegistryPATH()
}

// mergeRegistryPATH reads PATH from the registry (system + user) and merges
// any missing directories into the current process's PATH environment variable.
func mergeRegistryPATH() {
	current := os.Getenv("PATH")
	currentSet := pathSet(current)

	var extra []string

	// 1. System PATH: HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment
	if sysPath, err := readRegString(
		registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`,
		"Path",
	); err == nil {
		for _, dir := range splitPath(sysPath) {
			if _, ok := currentSet[strings.ToLower(dir)]; !ok {
				extra = append(extra, dir)
				currentSet[strings.ToLower(dir)] = struct{}{}
			}
		}
	}

	// 2. User PATH: HKCU\Environment
	if userPath, err := readRegString(
		registry.CURRENT_USER,
		`Environment`,
		"Path",
	); err == nil {
		for _, dir := range splitPath(userPath) {
			if _, ok := currentSet[strings.ToLower(dir)]; !ok {
				extra = append(extra, dir)
				currentSet[strings.ToLower(dir)] = struct{}{}
			}
		}
	}

	if len(extra) > 0 {
		newPath := current
		if newPath != "" {
			newPath += ";"
		}
		newPath += strings.Join(extra, ";")
		_ = os.Setenv("PATH", newPath)
	}
}

// readRegString reads a string value from the Windows registry.
// It handles both REG_SZ and REG_EXPAND_SZ (expanding %VAR% references).
func readRegString(root registry.Key, subKey, valueName string) (string, error) {
	k, err := registry.OpenKey(root, subKey, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	val, valType, err := k.GetStringValue(valueName)
	if err != nil {
		return "", err
	}

	// Expand environment variables like %SystemRoot% for REG_EXPAND_SZ
	if valType == registry.EXPAND_SZ {
		val = expandEnv(val)
	}
	return val, nil
}

// expandEnv expands %VAR% style environment variable references in a string.
func expandEnv(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '%' {
			j := strings.Index(s[i+1:], "%")
			if j >= 0 {
				varName := s[i+1 : i+1+j]
				if val := os.Getenv(varName); val != "" {
					result.WriteString(val)
				} else {
					result.WriteString(s[i : i+2+j]) // keep original if not found
				}
				i = i + 2 + j
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// splitPath splits a PATH string by semicolons, trimming whitespace and
// skipping empty entries.
func splitPath(path string) []string {
	var dirs []string
	for _, d := range strings.Split(path, ";") {
		d = strings.TrimSpace(d)
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// pathSet builds a case-insensitive set of directories from a PATH string.
func pathSet(path string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, d := range splitPath(path) {
		m[strings.ToLower(d)] = struct{}{}
	}
	return m
}
