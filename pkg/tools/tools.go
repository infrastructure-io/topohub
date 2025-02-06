package tools

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

// ValidateIPRangeExpansion 检查新的 IP 范围是否完全覆盖了旧的 IP 范围
// 返回 error 如果新范围缩小了任何一个旧范围
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

// CompareIP compares two IP addresses
// Returns:
//   -1 if ip1 < ip2
//    0 if ip1 == ip2
//    1 if ip1 > ip2
func CompareIP(ip1, ip2 net.IP) int {
	return bytes.Compare(ip1, ip2)
}

// Int32PtrEqual compares two *int32 values for equality
func Int32PtrEqual(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// CountIPsInRange 计算 IP 范围中包含的 IP 地址数量
// 对于 "192.168.1.1-192.168.1.10" 返回 10
// 对于 "192.168.1.1" 返回 1
// 对于 "192.168.1.1-192.168.1.10,192.168.1.20" 返回 11
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

// ipToUint32 将 IPv4 地址转换为 uint32
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}
