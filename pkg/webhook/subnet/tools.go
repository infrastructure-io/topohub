package subnet

import (
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidateIPInSubnet checks if an IP address is within a subnet
func ValidateIPInSubnet(ip net.IP, subnet *net.IPNet) bool {
	return subnet.Contains(ip)
}

// ValidateIPWithSubnetMatch checks if an IP/CIDR has the same subnet as the given subnet
func ValidateIPWithSubnetMatch(ipCIDR string, subnet *net.IPNet) error {
	ip, ipNet, err := net.ParseCIDR(ipCIDR)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %v", err)
	}

	// Check if IP is in subnet
	if !ValidateIPInSubnet(ip, subnet) {
		return fmt.Errorf("IP %s is not in subnet %s", ip, subnet)
	}

	// Get the network part of the IP using the subnet mask
	ipNetValue := net.IPNet{
		IP:   ip.Mask(subnet.Mask),
		Mask: subnet.Mask,
	}
	ipNet = &ipNetValue

	// Compare the network parts
	if !ipNet.IP.Equal(subnet.IP.Mask(subnet.Mask)) {
		return fmt.Errorf("IP %s does not match subnet network %s", ip, subnet)
	}

	return nil
}

// ValidateIPRange checks if all IPs in a range are within a subnet
func ValidateIPRange(ipRange string, subnet *net.IPNet) error {
	ranges := strings.Split(ipRange, ",")
	for _, r := range ranges {
		if strings.Contains(r, "-") {
			// Range format: start-end
			startEnd := strings.Split(r, "-")
			if len(startEnd) != 2 {
				return fmt.Errorf("invalid IP range format: %s", r)
			}

			start := net.ParseIP(strings.TrimSpace(startEnd[0]))
			end := net.ParseIP(strings.TrimSpace(startEnd[1]))

			if start == nil || end == nil {
				return fmt.Errorf("invalid IP address in range: %s", r)
			}

			if !ValidateIPInSubnet(start, subnet) {
				return fmt.Errorf("start IP %s is not within subnet %s", start, subnet)
			}
			if !ValidateIPInSubnet(end, subnet) {
				return fmt.Errorf("end IP %s is not within subnet %s", end, subnet)
			}

			// Verify that start IP is less than end IP
			if bytes.Compare(start, end) > 0 {
				return fmt.Errorf("start IP %s is greater than end IP %s", start, end)
			}
		} else {
			// Single IP
			ip := net.ParseIP(strings.TrimSpace(r))
			if ip == nil {
				return fmt.Errorf("invalid IP address: %s", r)
			}
			if !ValidateIPInSubnet(ip, subnet) {
				return fmt.Errorf("IP %s is not within subnet %s", ip, subnet)
			}
		}
	}
	return nil
}

// IsValidInterfaceName checks if the interface name is valid
func IsValidInterfaceName(name string) bool {
	// Interface name validation based on Linux interface naming conventions
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched && len(name) <= 15
}

// ValidateInterfaceExists checks if a network interface exists on the system
func ValidateInterfaceExists(ifaceName string) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %v", err)
	}

	for _, iface := range ifaces {
		if iface.Name == ifaceName {
			return nil
		}
	}

	return fmt.Errorf("interface %s does not exist on the system", ifaceName)
}
