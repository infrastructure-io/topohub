// Package dhcpserver defines the common types used by the DHCP server
package dhcpserver

// DhcpClientInfo represents information about a DHCP client
type DhcpClientInfo struct {
	MAC         string `json:"mac"`
	IP          string `json:"ip"`
	Active      bool   `json:"active"`
	StartTime   string `json:"startTime"`
	Subnet      string `json:"subnet"`
	ClusterName string `json:"clusterName,omitempty"`
}

// IPUsageStats represents IP usage statistics for a subnet
type IPUsageStats struct {
	TotalIPs     int `json:"totalIPs"`
	UsedIPs      int `json:"usedIPs"`
	AvailableIPs int `json:"availableIPs"`
}

// DhcpServerConfig represents the configuration for the DHCP server
type DhcpServerConfig struct {
	Interface string
	VlanID    *int32
	SelfIP    string
	Subnet    string
	IPRanges  []string
	Gateway   *string
	DNS       *string
}
