//go:build windows

package wsl

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	pkgexec "go-port-forward/pkg/os/exec"
)

// psUTF8Prefix forces PowerShell to output UTF-8 so we avoid UTF-16 BOM issues.
const psUTF8Prefix = "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; "

var reWSLDistroLine = regexp.MustCompile(`^\s*(\*?)\s*(.*?)\s{2,}(\S+)\s+(\S+)\s*$`)

// findPowerShell returns the path to the best available PowerShell executable.
// It prefers the modern cross-platform "pwsh" over the legacy "powershell".
func findPowerShell() string {
	if p, err := exec.LookPath("pwsh"); err == nil {
		return p
	}
	if p, err := exec.LookPath("powershell"); err == nil {
		return p
	}
	return "powershell" // fallback, let the OS resolve it
}

// ListDistros returns all installed WSL distributions.
func ListDistros() ([]Distro, error) {
	ps := findPowerShell()

	// Force UTF-8 output to avoid UTF-16 LE BOM mangling the result.
	out, err := pkgexec.ExecCommand(ps, "-NoProfile", "-NonInteractive", "-Command",
		psUTF8Prefix+"wsl --list --verbose").Output()
	if err != nil {
		return nil, fmt.Errorf("wsl list: %w", err)
	}

	// Strip UTF-8 BOM if present
	raw := strings.TrimPrefix(string(out), "\xef\xbb\xbf")

	var distros []Distro
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if i == 0 { // header
			continue
		}
		// Strip any remaining null bytes from UTF-16 conversion remnants
		line = strings.ReplaceAll(line, "\x00", "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := reWSLDistroLine.FindStringSubmatch(line)
		if len(m) != 5 {
			continue
		}
		distros = append(distros, Distro{
			Name:    strings.TrimSpace(m[2]),
			State:   m[3],
			Version: m[4],
			Default: m[1] == "*",
		})
	}
	return distros, nil
}

// GetIP returns the IP address of a WSL2 distribution.
// Uses ExecuteInWSL for reliable UTF-16/UTF-8 handling.
func GetIP(distro string) (string, error) {
	result, err := ExecuteInWSL(distro, "hostname", "-I")
	if err != nil {
		// Fallback: query via PowerShell
		ps := findPowerShell()
		quotedDistro := quotePowerShellLiteral(distro)
		out, err2 := pkgexec.ExecCommand(ps, "-NoProfile", "-NonInteractive", "-Command",
			psUTF8Prefix+"wsl -d "+quotedDistro+" -- hostname -I").Output()
		if err2 != nil {
			return "", fmt.Errorf("wsl get IP for %s: %w", distro, err)
		}
		result = strings.TrimSpace(strings.ReplaceAll(string(out), "\x00", ""))
	}
	ip := strings.Fields(result)
	if len(ip) == 0 {
		return "", fmt.Errorf("no IP found for distro %s", distro)
	}
	return ip[0], nil
}

// ListPorts returns TCP and UDP ports that are listening inside a WSL2 distro.
func ListPorts(distro string) ([]Port, error) {
	tcpPorts, err := listPortsByProto(distro, "tcp")
	if err != nil {
		return nil, err
	}
	udpPorts, err := listPortsByProto(distro, "udp")
	if err != nil {
		return nil, err
	}
	return append(tcpPorts, udpPorts...), nil
}

// reSSProcess extracts the process name from ss -p output like: users:(("nginx",pid=1,fd=4))
var reSSProcess = regexp.MustCompile(`users:\(\("([^"]+)"`)

func listPortsByProto(distro, proto string) ([]Port, error) {
	flag := "-tlnp"
	if proto == "udp" {
		flag = "-ulnp"
	}
	// Try "ss" first (available if /usr/sbin is in PATH), then fall back to full path
	result, err := ExecuteInWSL(distro, "ss", flag)
	if err != nil {
		// Fallback: use absolute path /usr/sbin/ss (common location on Debian/Ubuntu)
		result, err = ExecuteInWSL(distro, "/usr/sbin/ss", flag)
		if err != nil {
			return nil, fmt.Errorf("wsl ss %s for %s: %w", proto, distro, err)
		}
	}

	var ports []Port
	seen := make(map[int]bool) // deduplicate by port number (ss returns both IPv4 and IPv6 entries)
	lines := strings.Split(strings.ReplaceAll(result, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// Local Address:Port is the 4th column (index 3)
		localAddr := fields[3]
		// Handle "addr:port", "*:port", and "[::]:port" forms
		lastColon := strings.LastIndex(localAddr, ":")
		if lastColon < 0 {
			continue
		}
		portStr := localAddr[lastColon+1:]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		// Skip duplicate port entries (e.g., *:8080 and [::]:8080 for the same listener)
		if seen[port] {
			continue
		}
		seen[port] = true

		// Extract process name from users:(("nginx",pid=1,fd=4)) pattern
		process := ""
		for _, f := range fields[5:] {
			if m := reSSProcess.FindStringSubmatch(f); len(m) >= 2 {
				process = m[1]
				break
			}
			if process == "" && strings.HasPrefix(f, "users:") {
				process = f
			}
		}

		ports = append(ports, Port{
			Protocol:  proto,
			Port:      port,
			Process:   process,
			LocalAddr: localAddr,
		})
	}
	return ports, nil
}

func quotePowerShellLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
