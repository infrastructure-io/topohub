// Package types defines the common types used by the DHCP server
package dhcpserver

// DhcpClientInfo represents information about a DHCP client lease
type DhcpClientInfo struct {
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

	// Subnet is the subnet where the client is allocated (in format "192.168.1.0/24")
	Subnet string

	// ClusterName is the name of the cluster where the client is allocated
	ClusterName string
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
