//go:build !linux
// +build !linux

package dhcpserver

import "github.com/vishvananda/netlink"

func (s *dhcpServer) configureIP(name, ipStr string) error {
	return netlink.ErrNotImplemented
}
