package dhcpserver

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// configureIP configures IP address on the interface
func (s *dhcpServer) configureIP(name, ipStr string) error {
	s.log.Infof("Configuring IP address: %s on interface %s", ipStr, name)

	// 获取接口
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface %s not found: %v", name, err)
	}

	// 解析 IP 地址
	addr, err := netlink.ParseAddr(ipStr)
	if err != nil {
		return fmt.Errorf("invalid IP address %s: %v", ipStr, err)
	}

	// 检查是否已经配置了该 IP
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %v", err)
	}

	for _, existing := range addrs {
		if existing.Equal(*addr) {
			s.log.Debugf("IP %s already configured on %s", ipStr, name)
			return nil
		}
	}

	// 添加 IP 地址
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add IP address: %v", err)
	}

	return nil
}
