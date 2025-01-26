// Package dhcpserver provides DHCP server management functionality.
package dhcpserver

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"text/template"

	hoststatusdata "github.com/infrastructure-io/topohub/pkg/agent/hoststatus/data"
	"github.com/infrastructure-io/topohub/pkg/dhcpserver/types"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/vishvananda/netlink"
)

const (
	virtualMac = "00:00:00:00:00:00"
)

type FixedIP struct {
	IpAddr string
	Mac    string
}

// configureInterface configures the network interface with the specified IP address
// Parameters:
//   - interfaceName: name of the network interface to configure
//   - selfIP: IP address to assign to the interface
//
// Returns an error if interface configuration fails.
func configureInterface(interfaceName, selfIP string) error {
	// Get the network interface by name
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		log.Logger.Errorf("failed to get interface %s: %v", interfaceName, err)
		return err
	}

	// Ensure interface is up
	if err := netlink.LinkSetUp(link); err != nil {
		log.Logger.Errorf("failed to set interface %s up: %v", interfaceName, err)
		return err
	}

	// Get existing addresses
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		log.Logger.Errorf("failed to get addresses for interface %s: %v", interfaceName, err)
		return err
	}

	// Iterate over existing IP addresses and remove them
	for _, addr := range addrs {
		log.Logger.Debugf("Removing IP address %s from interface %s", addr.IP, interfaceName)
		if err := netlink.AddrDel(link, &addr); err != nil {
			log.Logger.Errorf("failed to remove IP address %s: %v", addr.IP, err)
			return err
		}
	}

	// Parse and add the new IP address
	addr, err := netlink.ParseAddr(selfIP)
	if err != nil {
		log.Logger.Errorf("failed to parse IP address %s: %v", selfIP, err)
		return err
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		log.Logger.Errorf("failed to add IP address %s: %v", selfIP, err)
		return err
	}
	log.Logger.Debugf("Added IP address %s to interface %s", selfIP, interfaceName)

	return nil
}

// generateConfig generates the DHCP server configuration file
// Parameters:
//   - s.config.Subnet: subnet in CIDR notation that the DHCP server will serve
//   - s.config.IpRange: range of IP addresses for allocation
//   - s.config.Gateway: gateway IP address for the subnet
//
// Returns an error if configuration file cannot be created or written.
func (s *dhcpServer) generateConfig() error {
	s.lastBoundIPLock.Lock()
	defer s.lastBoundIPLock.Unlock()

	// Get network and mask from subnet
	network, netmask, err := getNetworkAndMask(s.config.Subnet)
	if err != nil {
		return fmt.Errorf("failed to get network and mask: %v", err)
	}

	// Read template file
	tmplContent, err := os.ReadFile(DhcpConfigTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read DHCP config template: %v", err)
	}

	// Parse template
	tmpl, err := template.New("dhcp-config").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse DHCP config template: %v", err)
	}

	// Convert IP range format from "start-end" to "start end"
	ipRange := strings.Replace(s.config.IpRange, "-", " ", 1)

	// Prepare template data
	data := struct {
		Subnet     string
		Netmask    string
		Range      string
		Router     string
		SubnetMask string
		FixedIPs   map[string]FixedIP
	}{
		Subnet:     network,
		Netmask:    netmask,
		Range:      ipRange,
		Router:     s.config.Gateway,
		SubnetMask: netmask,
		FixedIPs:   make(map[string]FixedIP),
	}

	// Create config file
	f, err := os.Create(s.dhcpConfigPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer f.Close()

	fixedIPs := map[string]string{}
	if s.config.EnableBindStaticIP {
		dhcpclientList := hoststatusdata.HostCacheDatabase.GetDhcpClientInfo()
		for _, client := range dhcpclientList {
			fixedIPs[client.Info.IpAddr] = client.Info.Mac
		}
	}
	if s.config.EnableBindStaticIP {
		staticclientList := hoststatusdata.HostCacheDatabase.GetStaticClientInfo()
		for _, client := range staticclientList {
			fixedIPs[client.Info.IpAddr] = virtualMac
		}
	}

	// record the bound ip for this boot
	s.lastBoundIPList = fixedIPs

	if len(fixedIPs) > 0 {

		// 将 fixedIPs 转换为 map 格式，并检查 IP 是否属于子网
		_, subnet, err := net.ParseCIDR(s.config.Subnet)
		if err != nil {
			return fmt.Errorf("failed to parse subnet CIDR: %v", err)
		}

		count := 0
		for ip, mac := range fixedIPs {
			// 检查 IP 是否属于子网
			ipAddr := net.ParseIP(ip)
			if ipAddr == nil {
				log.Logger.Warnf("invalid IP address format: %s, skipping", ip)
				continue
			}

			if !subnet.Contains(ipAddr) {
				log.Logger.Debugf("IP %s is not in subnet %s, skipping", ip, s.config.Subnet)
				continue
			}

			count++
			hostName := fmt.Sprintf("host%d", count)
			data.FixedIPs[hostName] = FixedIP{IpAddr: ip, Mac: mac}
			log.Logger.Debugf("add fixed ip to dhcp configuration for dhcp client: ip %s, mac %s", ip, mac)
		}
	}

	// Execute template
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}
	f.Close()

	// Read the generated config file for logging
	content, err := os.ReadFile(s.dhcpConfigPath)
	if err != nil {
		log.Logger.Warnf("Failed to read generated config file for logging: %v", err)
		return err
	} else {
		log.Logger.Infof("generated DHCP config file at %s:\n%s", s.dhcpConfigPath, string(content))
	}

	return nil
}

// calculateTotalIPs computes the total number of IP addresses available for allocation
// Parameters:
//   - s.config.IpRange: range of IP addresses in format "start end" (e.g., "192.168.1.100 192.168.1.200")
//
// Returns:
//   - s.totalIPs: number of IP addresses in the range
//   - err: error if IP range is invalid or calculation fails
func (s *dhcpServer) calculateTotalIPs() error {
	// Split IP range string into start and end IP addresses
	rangeParts := strings.Split(s.config.IpRange, "-")
	if len(rangeParts) != 2 {
		log.Logger.Errorf("invalid IP range format: %s", s.config.IpRange)
		return fmt.Errorf("invalid IP range format: %s", s.config.IpRange)
	}

	// Parse start and end IP addresses
	startIP := net.ParseIP(strings.TrimSpace(rangeParts[0]))
	endIP := net.ParseIP(strings.TrimSpace(rangeParts[1]))
	if startIP == nil || endIP == nil {
		log.Logger.Errorf("invalid IP addresses in range: %s", s.config.IpRange)
		return fmt.Errorf("invalid IP addresses in range: %s", s.config.IpRange)
	}

	// Calculate total number of IP addresses in the range
	s.totalIPs = ipRange(startIP, endIP)
	log.Logger.Debugf("Calculated total IPs in range %s: %d", s.config.IpRange, s.totalIPs)
	return nil
}

// ipRange calculates the number of IPs in a range
// Parameters:
//   - start: first IP address in the range
//   - end: last IP address in the range
//
// Returns:
//   - total number of IP addresses in the range
func ipRange(start, end net.IP) int {
	// Convert IP addresses to integers
	startInt := ipToInt(start)
	endInt := ipToInt(end)

	// Calculate total number of IP addresses in the range
	return int(endInt - startInt + 1)
}

// ipToInt converts an IP address to an integer
// Parameters:
//   - ip: IP address to convert
//
// Returns:
//   - integer representation of the IP address
func ipToInt(ip net.IP) int64 {
	// Convert IP address to IPv4 format
	ip = ip.To4()
	if ip == nil {
		return 0
	}

	// Convert IP address to integer
	return int64(ip[0])<<24 | int64(ip[1])<<16 | int64(ip[2])<<8 | int64(ip[3])
}

// getNetworkAndMask returns the network and mask from a CIDR notation string
func getNetworkAndMask(cidr string) (string, string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", err
	}

	// Convert netmask from IPMask format to dotted decimal format
	mask := fmt.Sprintf("%d.%d.%d.%d",
		ipNet.Mask[0], ipNet.Mask[1], ipNet.Mask[2], ipNet.Mask[3])

	return ipNet.IP.String(), mask, nil
}

// GetDhcpClients parses the DHCP lease file and returns information about active DHCP clients.
// If the lease file doesn't exist or is empty, returns an empty slice.
func GetDhcpClients(leaseFilePath string) ([]types.ClientInfo, error) {
	content, err := os.ReadFile(leaseFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	log.Logger.Debugf("Parsing DHCP lease file content:\n%s", string(content))

	var clients []types.ClientInfo
	lines := strings.Split(string(content), "\n")
	var currentClient *types.ClientInfo
	var inLeaseBlock bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		//log.Logger.Debugf("Processing line: %s", line)

		// Start of a lease block
		if strings.HasPrefix(line, "lease") {
			if currentClient != nil {
				log.Logger.Debugf("Adding client: IP=%s, MAC=%s, Active=%v",
					currentClient.IP, currentClient.MAC, currentClient.Active)
				clients = append(clients, *currentClient)
			}
			currentClient = &types.ClientInfo{}
			inLeaseBlock = true

			// Extract IP address
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				currentClient.IP = strings.TrimSpace(strings.TrimSuffix(parts[1], "{"))
				log.Logger.Debugf("Found lease for IP: %s", currentClient.IP)
			}
			continue
		}

		// End of a lease block
		if line == "}" {
			if currentClient != nil {
				clients = append(clients, *currentClient)
				currentClient = nil
			}
			inLeaseBlock = false
			continue
		}

		// Inside a lease block
		if inLeaseBlock && currentClient != nil {
			// Extract MAC address
			if strings.Contains(line, "hardware ethernet") {
				parts := strings.Split(line, "hardware ethernet")
				if len(parts) == 2 {
					currentClient.MAC = strings.TrimSpace(strings.TrimSuffix(parts[1], ";"))
				}
			}

			// Extract binding state
			if strings.Contains(line, "binding state") {
				parts := strings.Split(line, "binding state")
				if len(parts) == 2 {
					state := strings.TrimSpace(strings.TrimSuffix(parts[1], ";"))
					currentClient.Active = (state == "active")
				}
			}

			// Extract start time
			if strings.Contains(line, "starts") {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					currentClient.StartTime = strings.TrimSuffix(parts[2]+" "+parts[3], ";")
				}
			}

			// Extract end time
			if strings.Contains(line, "ends") {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					currentClient.EndTime = strings.TrimSuffix(parts[2]+" "+parts[3], ";")
				}
			}
		}
	}

	// Handle the last lease block if exists
	if currentClient != nil {
		log.Logger.Debugf("Adding final client: IP=%s, MAC=%s, Active=%v",
			currentClient.IP, currentClient.MAC, currentClient.Active)
		clients = append(clients, *currentClient)
	}

	log.Logger.Debugf("Total clients found: %d", len(clients))
	return clients, nil
}

// printDhcpLogTail prints the last 50 lines of the DHCP server log file
func (s *dhcpServer) printDhcpLogTail() error {
	// Check if log file exists
	if _, err := os.Stat(DhcpLogFile); os.IsNotExist(err) {
		return fmt.Errorf("DHCP log file not found: %s", DhcpLogFile)
	}

	// Use tail command to get last 50 lines
	cmd := exec.Command("tail", "-n", "50", DhcpLogFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to tail DHCP log file: %v", err)
	}

	// Print the log lines with a header
	log.Logger.Info("=== Last 50 lines of DHCP server log ===")
	log.Logger.Info(string(output))
	log.Logger.Info("=======================================")

	return nil
}
