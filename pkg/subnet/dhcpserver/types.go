// Package dhcpserver defines the common types used by the DHCP server
package dhcpserver

import "time"

// DhcpClientInfo represents information about a DHCP client
type DhcpClientInfo struct {
	MAC            string    `json:"mac"`
	IP             string    `json:"ip"`
	Hostname       string    `json:"hostname"`
	Active         bool      `json:"active"`
	DhcpExpireTime time.Time `json:"dhcpExpireTime"` // When the DHCP lease expires
	Subnet         string    `json:"subnet"`
	SubnetName     string    `json:"subnetName"`
	ClusterName    string    `json:"clusterName,omitempty"`
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
