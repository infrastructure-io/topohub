// Package types defines the common types used by the DHCP server
package types

// ClientInfo represents information about a DHCP client lease
type ClientInfo struct {
	// IP is the allocated IP address for the client
	IP string
	// MAC is the client's MAC address
	MAC string
	// Active indicates whether this lease is in active state
	Active bool
	// StartTime is when the lease starts (in format "2024/12/18 10:00:00")
	StartTime string
	// EndTime is when the lease ends (in format "2024/12/18 10:30:00")
	EndTime string
}

// IPUsageStats represents statistics about IP address allocation
type IPUsageStats struct {
	// UsedIPs is the number of IP addresses currently in use
	UsedIPs int
	// AvailableIPs is the number of IP addresses currently available
	AvailableIPs int
	// UsagePercentage is the percentage of IP addresses currently in use
	UsagePercentage float64
}

// DhcpServerConfig represents the configuration for the DHCP server
type DhcpServerConfig struct {
	// Interface is the network interface to listen on
	Interface string
	// SelfIP is the IP address to assign to the interface
	SelfIP string
	// StartIP is the start of the IP range
	StartIP string
	// EndIP is the end of the IP range
	EndIP string
	// Netmask is the subnet mask
	Netmask string
	// Gateway is the default gateway
	Gateway string
	// LeaseTime is the lease duration in seconds
	LeaseTime int64
}

// DhcpServer defines the interface for DHCP server operations.
// This interface provides methods to control the DHCP server and retrieve information
// about its current state.
type DhcpServer interface {
	// Start initializes and starts the DHCP server.
	// It configures the network interface if SelfIp is specified,
	// generates the DHCP configuration, and starts the dhcpd process.
	// Returns an error if any step fails.
	Start() error

	// Stop gracefully stops the DHCP server.
	// It terminates the dhcpd process and cleans up any monitoring routines.
	// Returns an error if the server cannot be stopped.
	Stop() error

	// GetClientInfo retrieves information about all current DHCP clients.
	// Returns a list of ClientInfo containing IP and MAC addresses for each client,
	// or an error if the information cannot be retrieved.
	GetClientInfo() ([]ClientInfo, error)

	// GetIPUsageStats retrieves current IP allocation statistics.
	// Returns IPUsageStats containing total and available IP counts,
	// or an error if the statistics cannot be calculated.
	GetIPUsageStats() (*IPUsageStats, error)
}
