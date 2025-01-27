package dhcpserver

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
	GetClientInfo() ([]DhcpClientInfo, error)

	// GetIPUsageStats retrieves current IP allocation statistics.
	// Returns IPUsageStats containing total and available IP counts,
	// or an error if the statistics cannot be calculated.
	GetIPUsageStats() (*IPUsageStats, error)
}
