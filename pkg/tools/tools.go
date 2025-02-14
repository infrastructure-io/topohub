package tools

import (
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidateIPInSubnet checks if an IP address is within a subnet
// Example:
//   - Input:
//     ip: net.ParseIP("192.168.1.100")
//     subnet: net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)}
//   - Returns: true (because 192.168.1.100 is within 192.168.1.0/24)
func ValidateIPInSubnet(ip net.IP, subnet *net.IPNet) bool {
	return subnet.Contains(ip)
}

// ValidateIPWithSubnetMatch checks if an IP/CIDR has the same subnet as the given subnet
// Example:
//   - Input:
//     ipCIDR: "192.168.1.100/24"
//     subnet: net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)}
//   - Returns: nil (because 192.168.1.100/24 matches subnet 192.168.1.0/24)
//   - Error case: Returns error if IP is not in subnet or network parts don't match
func ValidateIPWithSubnetMatch(ipCIDR string, subnet *net.IPNet) error {
	ip, _, err := net.ParseCIDR(ipCIDR)
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

	// Compare the network parts
	if !ipNetValue.IP.Equal(subnet.IP.Mask(subnet.Mask)) {
		return fmt.Errorf("IP %s does not match subnet network %s", ip, subnet)
	}

	return nil
}

// ValidateIPRange checks if all IPs in a range are within a subnet
// Example:
//   - Input:
//     ipRange: "192.168.1.10-192.168.1.20,192.168.1.30"
//     subnet: net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)}
//   - Returns: nil if all IPs are within subnet
//   - Error case: Returns error if any IP is outside subnet or format is invalid
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

// ValidateIPRangeExpansion checks if new IP range fully covers the old IP range
// Example:
//   - Input:
//     oldIPRange: "192.168.1.10-192.168.1.20,192.168.1.30"
//     newIPRange: "192.168.1.5-192.168.1.25"
//     subnet: net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)}
//   - Returns: nil (because new range fully covers old range)
//   - Error case: Returns error if new range shrinks any part of old range
func ValidateIPRangeExpansion(oldIPRange, newIPRange string, subnet *net.IPNet) error {
	// 首先验证新旧 IP 范围的格式
	if err := ValidateIPRange(oldIPRange, subnet); err != nil {
		return fmt.Errorf("invalid old IP range: %v", err)
	}
	if err := ValidateIPRange(newIPRange, subnet); err != nil {
		return fmt.Errorf("invalid new IP range: %v", err)
	}

	// 分割新旧 IP 范围
	oldRanges := strings.Split(oldIPRange, ",")
	newRanges := strings.Split(newIPRange, ",")

	// 检查旧范围中的每个 IP 或 IP 范围是否都被新范围覆盖
	for _, oldRange := range oldRanges {
		oldRange = strings.TrimSpace(oldRange)
		var start, end net.IP

		if strings.Contains(oldRange, "-") {
			// IP 范围格式
			parts := strings.Split(oldRange, "-")
			start = net.ParseIP(strings.TrimSpace(parts[0]))
			end = net.ParseIP(strings.TrimSpace(parts[1]))
		} else {
			// 单个 IP 格式
			start = net.ParseIP(oldRange)
			end = start
		}

		// 检查这个范围是否被新范围覆盖
		covered := false
		for _, newRange := range newRanges {
			newRange = strings.TrimSpace(newRange)
			var newStart, newEnd net.IP

			if strings.Contains(newRange, "-") {
				parts := strings.Split(newRange, "-")
				newStart = net.ParseIP(strings.TrimSpace(parts[0]))
				newEnd = net.ParseIP(strings.TrimSpace(parts[1]))
			} else {
				newStart = net.ParseIP(newRange)
				newEnd = newStart
			}

			// 如果新范围覆盖了旧范围
			if CompareIP(newStart, start) <= 0 && CompareIP(newEnd, end) >= 0 {
				covered = true
				break
			}
		}

		if !covered {
			return fmt.Errorf("IP range cannot be shrunk. The range %s is not fully covered in the new configuration", oldRange)
		}
	}

	return nil
}

// IsValidInterfaceName checks if the interface name is valid according to Linux naming conventions
// Example:
//   - Input: "eth0" -> Returns: true
//   - Input: "my@interface" -> Returns: false
//   - Rules: Must be alphanumeric with underscore/hyphen, max 15 chars
func IsValidInterfaceName(name string) bool {
	// Interface name validation based on Linux interface naming conventions
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched && len(name) <= 15
}

// ValidateInterfaceExists checks if a network interface exists on the system
// Example:
//   - Input: "eth0"
//   - Returns: nil if interface exists
//   - Error case: Returns error if interface doesn't exist or system error occurs
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

// CompareIP compares two IP addresses
// Example:
//   - Input:
//     ip1: net.ParseIP("192.168.1.1")
//     ip2: net.ParseIP("192.168.1.2")
//   - Returns: -1 (because 192.168.1.1 < 192.168.1.2)
//   - Return values: -1 (ip1 < ip2), 0 (ip1 == ip2), 1 (ip1 > ip2)
func CompareIP(ip1, ip2 net.IP) int {
	return bytes.Compare(ip1, ip2)
}

// Int32PtrEqual compares two *int32 values for equality
// Example:
//   - Input:
//     a: pointer to int32(5)
//     b: pointer to int32(5)
//   - Returns: true (because values are equal)
//   - Special cases: Returns true if both nil, false if only one is nil
func Int32PtrEqual(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// CountIPsInRange calculates the number of IP addresses in a given range
// Example:
//   - Input: "192.168.1.1-192.168.1.10,192.168.1.20"
//   - Returns: 11 (10 IPs from range + 1 single IP)
//   - Error case: Returns error if range format is invalid
func CountIPsInRange(ipRange string) (uint64, error) {
	ranges := strings.Split(ipRange, ",")
	var total uint64 = 0

	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if strings.Contains(r, "-") {
			// IP 范围格式
			parts := strings.Split(r, "-")
			if len(parts) != 2 {
				return 0, fmt.Errorf("invalid IP range format: %s", r)
			}

			start := net.ParseIP(strings.TrimSpace(parts[0]))
			end := net.ParseIP(strings.TrimSpace(parts[1]))

			if start == nil || end == nil {
				return 0, fmt.Errorf("invalid IP address in range: %s", r)
			}

			// 确保 start 和 end 都是 IPv4 地址
			start = start.To4()
			end = end.To4()
			if start == nil || end == nil {
				return 0, fmt.Errorf("invalid IPv4 address in range: %s", r)
			}

			// 确保 start <= end
			if bytes.Compare(start, end) > 0 {
				return 0, fmt.Errorf("start IP %s is greater than end IP %s", start, end)
			}

			// 计算范围内的 IP 数量
			startInt := ipToUint32(start)
			endInt := ipToUint32(end)
			total += uint64(endInt - startInt + 1)
		} else {
			// 单个 IP
			ip := net.ParseIP(strings.TrimSpace(r))
			if ip == nil {
				return 0, fmt.Errorf("invalid IP address: %s", r)
			}
			ip = ip.To4()
			if ip == nil {
				return 0, fmt.Errorf("invalid IPv4 address: %s", r)
			}
			total++
		}
	}

	return total, nil
}

// IsValidIPv4 checks if a string represents a valid IPv4 address
// Example:
//   - Input: "192.168.1.1" -> Returns: true
//   - Input: "256.1.2.3" -> Returns: false
//   - Input: "192.168.1" -> Returns: false
//   - Input: "192.168.1.1.1" -> Returns: false
func IsValidIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	// Convert to IPv4 format and check if it's not nil
	return ip.To4() != nil
}

// IsValidUnicastMAC checks if a string represents a valid unicast MAC address
// Example:
//   - Input: "00:11:22:33:44:55" -> Returns: true
//   - Input: "01:00:5e:00:00:00" -> Returns: false (multicast)
//   - Input: "ff:ff:ff:ff:ff:ff" -> Returns: false (broadcast)
//   - Input: "00:11:22:33:44" -> Returns: false (wrong length)
//   - Input: "00-11-22-33-44-55" -> Returns: true (supports hyphen format)
//   - Input: "001122334455" -> Returns: true (supports no separator format)
func IsValidUnicastMAC(macStr string) bool {
	// Replace hyphens with colons for consistent parsing
	macStr = strings.ReplaceAll(macStr, "-", ":")
	
	// Handle MAC address without separators
	if len(macStr) == 12 && !strings.Contains(macStr, ":") {
		// Insert colons every 2 characters
		var buffer bytes.Buffer
		for i, char := range macStr {
			if i > 0 && i%2 == 0 {
				buffer.WriteRune(':')
			}
			buffer.WriteRune(char)
		}
		macStr = buffer.String()
	}
	
	// Parse MAC address
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return false
	}
	
	// Check if it's a 48-bit MAC address
	if len(mac) != 6 {
		return false
	}
	
	// Check if it's a unicast address (least significant bit of first octet is 0)
	return (mac[0] & 1) == 0
}

// IsIPInRange checks if an IP is within a given IP range string
// Example:
//   - Input:
//     ip: net.ParseIP("192.168.1.15")
//     ipRange: "192.168.1.10-192.168.1.20,192.168.1.30"
//   - Returns: true (because 192.168.1.15 is within 192.168.1.10-192.168.1.20)
//   - Input:
//     ip: net.ParseIP("192.168.1.30")
//     ipRange: "192.168.1.10-192.168.1.20,192.168.1.30"
//   - Returns: true (because 192.168.1.30 matches the single IP)
//   - Input:
//     ip: net.ParseIP("192.168.1.25")
//     ipRange: "192.168.1.10-192.168.1.20,192.168.1.30"
//   - Returns: false (because 192.168.1.25 is not in any range)
func IsIPInRange(ip net.IP, ipRange string) bool {
	ranges := strings.Split(ipRange, ",")
	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if strings.Contains(r, "-") {
			// Range format: start-end
			startEnd := strings.Split(r, "-")
			if len(startEnd) != 2 {
				continue
			}

			start := net.ParseIP(strings.TrimSpace(startEnd[0]))
			end := net.ParseIP(strings.TrimSpace(startEnd[1]))

			if start == nil || end == nil {
				continue
			}

			// Check if IP is within range
			if CompareIP(start, ip) <= 0 && CompareIP(ip, end) <= 0 {
				return true
			}
		} else {
			// Single IP format
			rangeIP := net.ParseIP(r)
			if rangeIP == nil {
				continue
			}
			// Check if IP matches exactly
			if ip.Equal(rangeIP) {
				return true
			}
		}
	}
	return false
}

// ipToUint32 converts an IPv4 address to uint32
// Example:
//   - Input: net.ParseIP("192.168.1.1")
//   - Returns: 3232235777 (binary: 11000000 10101000 00000001 00000001)
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}
