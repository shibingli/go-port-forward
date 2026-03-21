//go:build darwin

package firewall

import (
	"fmt"
	"strconv"
	"strings"

	pkgexec "go-port-forward/pkg/os/exec"
)

type darwinFirewall struct{}

// New returns the macOS firewall manager (uses pfctl).
func New() Manager { return &darwinFirewall{} }

// macOS pf anchor used for all managed rules.
const pfAnchor = "go-port-forward"

func (d *darwinFirewall) AddRule(r Rule) error {
	// Build a pf pass rule and load it into the named anchor.
	for _, proto := range protocols(r) {
		rule := fmt.Sprintf("pass in proto %s to any port %d\n", proto, r.Port)
		cmd := pkgexec.ExecCommand("pfctl", "-a", pfAnchor+"/"+ruleSuffix(r, proto), "-f", "-")
		cmd.Stdin = strings.NewReader(rule)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("pfctl add: %w — %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (d *darwinFirewall) DeleteRule(r Rule) error {
	for _, proto := range protocols(r) {
		anchor := pfAnchor + "/" + ruleSuffix(r, proto)
		// Flush the sub-anchor (effectively removes the rule)
		_ = pkgexec.ExecCommand("pfctl", "-a", anchor, "-F", "all").Run()
	}
	return nil
}

func (d *darwinFirewall) RuleExists(r Rule) (bool, error) {
	proto := "tcp"
	if r.Protocol == "udp" {
		proto = "udp"
	}
	anchor := pfAnchor + "/" + ruleSuffix(r, proto)
	out, err := pkgexec.ExecCommand("pfctl", "-a", anchor, "-sr").CombinedOutput()
	if err != nil {
		return false, nil
	}
	return strings.Contains(string(out), strconv.Itoa(r.Port)), nil
}

func ruleSuffix(r Rule, proto string) string {
	return fmt.Sprintf("%s-%d-%s", r.Name, r.Port, proto)
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
