//go:build linux

package firewall

import (
	"fmt"
	"strconv"
	"strings"

	pkgexec "go-port-forward/pkg/os/exec"
)

type linuxFirewall struct{}

// New returns the Linux firewall manager (uses iptables).
func New() Manager { return &linuxFirewall{} }

func (l *linuxFirewall) AddRule(r Rule) error {
	for _, proto := range protocols(r) {
		args := []string{"-A", "INPUT", "-p", proto,
			"--dport", strconv.Itoa(r.Port), "-j", "ACCEPT",
			"-m", "comment", "--comment", RuleName(r)}
		if out, err := pkgexec.ExecCommand("iptables", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("iptables add: %w — %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (l *linuxFirewall) DeleteRule(r Rule) error {
	for _, proto := range protocols(r) {
		args := []string{"-D", "INPUT", "-p", proto,
			"--dport", strconv.Itoa(r.Port), "-j", "ACCEPT",
			"-m", "comment", "--comment", RuleName(r)}
		_ = pkgexec.ExecCommand("iptables", args...).Run() // ignore error if rule not found
	}
	return nil
}

func (l *linuxFirewall) RuleExists(r Rule) (bool, error) {
	for _, proto := range protocols(r) {
		args := []string{"-C", "INPUT", "-p", proto,
			"--dport", strconv.Itoa(r.Port), "-j", "ACCEPT"}
		if err := pkgexec.ExecCommand("iptables", args...).Run(); err == nil {
			return true, nil
		}
	}
	return false, nil
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
