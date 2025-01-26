# DHCP Server Module

## Overview
The DHCP Server module provides a Go implementation for managing a DHCP server using the ISC DHCP daemon (dhcpd). It offers a clean interface for starting, stopping, and monitoring DHCP services, as well as managing IP address allocation.

## Features
- DHCP server management (start/stop)
- Network interface configuration
- IP address allocation monitoring
- Client lease tracking
- Automatic server recovery
- Detailed logging and statistics

## Components
- `interface.go`: Defines the public interfaces and data structures
- `server.go`: Core DHCP server implementation
- `tools.go`: Helper functions for network and IP management
- `constants.go`: Configuration constants and templates

## Configuration

### Environment Variables
  - All dhcpd output will be captured and logged through the BMC logging system
  - Stdout is logged at INFO level
  - Stderr is logged at ERROR level
  - Default: false (dhcpd logs only to its log file)

### DHCP Server Configuration
The module uses the `DhcpServerConfig` from the BMC API specification, which includes:
- DHCP server interface name
- Subnet configuration
- IP range for allocation
- Gateway address
- Optional self-IP for the DHCP interface

## Usage Example
```go
import (
    bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/apis/bmc/v1beta1"
    "github.com/infrastructure-io/topohub/pkg/dhcpserver"
)

// Create DHCP server configuration
config := &bmcv1beta1.DhcpServerConfig{
    DhcpServerInterface: "eth0",
    Subnet:             "192.168.1.0/24",
    IpRange:            "192.168.1.100-192.168.1.200",
    Gateway:            "192.168.1.1",
}

// Create and start DHCP server
server := dhcpserver.NewDhcpServer(config)
if err := server.Start(); err != nil {
    // Handle error
}

// Get current IP allocation statistics
stats, err := server.GetIPUsageStats()
if err != nil {
    // Handle error
}

// Get list of current DHCP clients
clients, err := server.GetClientInfo()
if err != nil {
    // Handle error
}

// Stop DHCP server
if err := server.Stop(); err != nil {
    // Handle error
}
```

## Dependencies
- ISC DHCP daemon (dhcpd)
- github.com/vishvananda/netlink for network interface management
- BMC logging package for structured logging

## Notes
- The module requires root privileges to:
  - Configure network interfaces
  - Start/stop the DHCP daemon
  - Access DHCP configuration and lease files
- The DHCP server is automatically monitored and restarted if it fails
- All operations are thread-safe
