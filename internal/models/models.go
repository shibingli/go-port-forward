package models

import (
	"fmt"
	"time"

	"go-port-forward/pkg/os/wsl"
)

// Protocol represents the network protocol for forwarding
type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolBoth Protocol = "both"
)

// RuleStatus represents the runtime status of a forwarding rule
type RuleStatus string

const (
	StatusActive   RuleStatus = "active"
	StatusInactive RuleStatus = "inactive"
	StatusError    RuleStatus = "error"
)

// ForwardRule represents a single port forwarding rule
type ForwardRule struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ListenAddr string   `json:"listen_addr"` // "" or "0.0.0.0" means all interfaces
	Protocol   Protocol `json:"protocol"`
	TargetAddr string   `json:"target_addr"`
	Comment    string   `json:"comment"`

	// Runtime stats — not persisted
	Status      RuleStatus `json:"status"`
	ErrorMsg    string     `json:"error_msg,omitempty"` // reason the forwarder failed to start
	ListenPort  int        `json:"listen_port"`
	TargetPort  int        `json:"target_port"`
	BytesIn     int64      `json:"bytes_in"`
	BytesOut    int64      `json:"bytes_out"`
	ActiveConns int64      `json:"active_conns"`
	TotalConns  int64      `json:"total_conns"`
	Enabled     bool       `json:"enabled"`
	AddFirewall bool       `json:"add_firewall"` // auto-add firewall rule on creation
}

// ListenKey returns a unique key for the listen address+port+protocol combination
func (r *ForwardRule) ListenKey() string {
	return fmt.Sprintf("%s:%d/%s", r.ListenAddr, r.ListenPort, r.Protocol)
}

// Stats represents aggregated statistics across all rules
type Stats struct {
	TotalRules    int   `json:"total_rules"`
	EnabledRules  int   `json:"enabled_rules"`
	ActiveRules   int   `json:"active_rules"`
	TotalBytesIn  int64 `json:"total_bytes_in"`
	TotalBytesOut int64 `json:"total_bytes_out"`
	TotalConns    int64 `json:"total_conns"`
}

// WSLDistro is a type alias for wsl.Distro (WSL2 distribution)
type WSLDistro = wsl.Distro

// WSLPort is a type alias for wsl.Port (WSL2 listening port)
type WSLPort = wsl.Port

// CreateRuleRequest is the API request for creating a new rule
type CreateRuleRequest struct {
	Name        string   `json:"name"`
	ListenAddr  string   `json:"listen_addr"`
	Protocol    Protocol `json:"protocol"`
	TargetAddr  string   `json:"target_addr"`
	Comment     string   `json:"comment"`
	ListenPort  int      `json:"listen_port"`
	TargetPort  int      `json:"target_port"`
	AddFirewall bool     `json:"add_firewall"`
	Enabled     bool     `json:"enabled"`
}

// UpdateRuleRequest is the API request for updating a rule
type UpdateRuleRequest struct {
	Name        *string   `json:"name"`
	ListenAddr  *string   `json:"listen_addr"`
	ListenPort  *int      `json:"listen_port"`
	Protocol    *Protocol `json:"protocol"`
	TargetAddr  *string   `json:"target_addr"`
	TargetPort  *int      `json:"target_port"`
	AddFirewall *bool     `json:"add_firewall"`
	Comment     *string   `json:"comment"`
	Enabled     *bool     `json:"enabled"`
}

// WSLImportRequest is the API request for importing WSL2 ports
type WSLImportRequest struct {
	Distro     string    `json:"distro"`
	TargetAddr string    `json:"target_addr"` // WSL2 IP to forward to
	Ports      []WSLPort `json:"ports"`
}

// APIResponse is a generic JSON API response wrapper
type APIResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Success bool        `json:"success"`
}
