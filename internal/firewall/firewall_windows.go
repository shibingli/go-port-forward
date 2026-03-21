//go:build windows

package firewall

import (
	"fmt"
	"strconv"
	"strings"

	pkgexec "go-port-forward/pkg/os/exec"
)

type windowsFirewall struct{}

// New returns the Windows Firewall manager (uses netsh advfirewall).
func New() Manager { return &windowsFirewall{} }

func (w *windowsFirewall) AddRule(r Rule) error {
	name := RuleName(r)
	protos := protocols(r)
	for _, proto := range protos {
		args := []string{
			"advfirewall", "firewall", "add", "rule",
			"name=" + name,
			"dir=in",
			"action=allow",
			"protocol=" + proto,
			"localport=" + strconv.Itoa(r.Port),
		}
		if out, err := pkgexec.ExecCommand("netsh", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("netsh add rule: %w — %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (w *windowsFirewall) DeleteRule(r Rule) error {
	name := RuleName(r)
	args := []string{"advfirewall", "firewall", "delete", "rule", "name=" + name}
	if out, err := pkgexec.ExecCommand("netsh", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("netsh delete rule: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (w *windowsFirewall) RuleExists(r Rule) (bool, error) {
	name := RuleName(r)
	args := []string{"advfirewall", "firewall", "show", "rule", "name=" + name}
	out, err := pkgexec.ExecCommand("netsh", args...).CombinedOutput()
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(out), name), nil
}

func protocols(r Rule) []string {
	switch r.Protocol {
	case "tcp":
		return []string{"tcp"}
	case "udp":
		return []string{"udp"}
	default:
		return []string{"tcp", "udp"}
	}
}
