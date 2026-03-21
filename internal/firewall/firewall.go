// Package firewall provides OS-agnostic firewall rule management.
package firewall

import "go-port-forward/internal/models"

// Rule represents a host firewall rule for a forwarding entry.
type Rule struct {
	Name     string
	Port     int
	Protocol models.Protocol
}

// Manager is the OS-specific firewall controller.
type Manager interface {
	// AddRule creates an inbound allow rule for the given port/protocol.
	AddRule(r Rule) error
	// DeleteRule removes the firewall rule for the given port/protocol.
	DeleteRule(r Rule) error
	// RuleExists returns true if the rule already exists.
	RuleExists(r Rule) (bool, error)
}

// RuleName generates a deterministic rule name.
func RuleName(r Rule) string {
	return "go-port-forward: " + r.Name
}
