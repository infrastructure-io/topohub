package subnet

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
)

// validateIPInSubnet checks if an IP address is within a subnet
func validateIPInSubnet(ip net.IP, subnet *net.IPNet) bool {
	return subnet.Contains(ip)
}

// validateIPWithSubnetMatch checks if an IP/CIDR has the same subnet as the given subnet
func validateIPWithSubnetMatch(ipCIDR string, subnet *net.IPNet) error {
	ip, ipNet, err := net.ParseCIDR(ipCIDR)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %v", err)
	}

	// Check if IP is in subnet
	if !validateIPInSubnet(ip, subnet) {
		return fmt.Errorf("IP %s is not in subnet %s", ip, subnet)
	}

	// Compare subnet masks
	if !ipNet.Mask.Equal(subnet.Mask) {
		return fmt.Errorf("subnet mask %s does not match required subnet mask %s", ipNet, subnet)
	}

	return nil
}

// validateIPRange checks if all IPs in a range are within a subnet
func validateIPRange(ipRange string, subnet *net.IPNet) error {
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

			if !validateIPInSubnet(start, subnet) {
				return fmt.Errorf("start IP %s is not within subnet %s", start, subnet)
			}
			if !validateIPInSubnet(end, subnet) {
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
			if !validateIPInSubnet(ip, subnet) {
				return fmt.Errorf("IP %s is not within subnet %s", ip, subnet)
			}
		}
	}
	return nil
}

// isValidInterfaceName checks if the interface name is valid
func isValidInterfaceName(name string) bool {
	// Interface name validation based on Linux interface naming conventions
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched && len(name) <= 15
}

// validateInterfaceExists checks if a network interface exists on the system
func validateInterfaceExists(ifaceName string) error {
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
