package dhcpserver

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// generateDnsmasqConfig generates the dnsmasq configuration file
func (s *dhcpServer) generateDnsmasqConfig(clients map[string]*DhcpClientInfo) (string, error) {

	s.log.Infof("generating config")

	// 读取模板文件
	tmpl, err := template.ParseFiles(s.configTemplatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	// 准备目录
	configFile := s.configPath
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %v", err)
	}

	// 准备接口名称
	var interfaceName string
	if s.subnet.Spec.Interface.VlanID != nil && *s.subnet.Spec.Interface.VlanID > 0 {
		interfaceName = fmt.Sprintf(vlanInterfaceFormat, s.subnet.Spec.Interface.Interface, *s.subnet.Spec.Interface.VlanID)
	} else {
		interfaceName = fmt.Sprintf(macvlanInterfaceFormat, s.subnet.Spec.Interface.Interface)
	}

	// 准备 DHCP 主机绑定
	var dhcpHosts []string
	if s.subnet.Spec.Feature.EnableBindDhcpIP && clients != nil {
		for _, client := range clients {
			dhcpHosts = append(dhcpHosts, fmt.Sprintf("dhcp-host=%s,%s", client.MAC, client.IP))
		}
	}

	data := struct {
		Interface        string
		IPRanges         []string
		Gateway          *string
		DNS              *string
		LeaseFile        string
		LogFile          string
		EnablePxe        bool
		EnableZtp        bool
		DhcpHostBindings []string
	}{
		Interface:        interfaceName,
		IPRanges:         strings.Split(s.subnet.Spec.IPv4Subnet.IPRange, ","),
		Gateway:          s.subnet.Spec.IPv4Subnet.Gateway,
		DNS:              s.subnet.Spec.IPv4Subnet.Dns,
		LeaseFile:        s.leasePath,
		LogFile:          s.logPath,
		EnablePxe:        s.subnet.Spec.Feature.EnablePxe,
		EnableZtp:        s.subnet.Spec.Feature.EnableZtp,
		DhcpHostBindings: dhcpHosts,
	}

	// 创建配置文件
	f, err := os.Create(configFile)
	if err != nil {
		return "", fmt.Errorf("failed to create config file: %v", err)
	}
	s.log.Infof("Generated dnsmasq config file: %s", configFile)
	defer f.Close()

	s.log.Debugf("Generated dnsmasq config: %+v", data)

	// 写入配置
	if err := tmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("failed to write config: %v", err)
	}

	// 统计 IP 使用情况
	totalIPs := 0
	for _, ipRange := range strings.Split(s.subnet.Spec.IPv4Subnet.IPRange, ",") {
		parts := strings.Split(ipRange, "-")
		if len(parts) != 2 {
			continue
		}
		start := net.ParseIP(parts[0])
		end := net.ParseIP(parts[1])
		if start == nil || end == nil {
			continue
		}
		totalIPs += int(binary.BigEndian.Uint32(end.To4())) - int(binary.BigEndian.Uint32(start.To4())) + 1
	}
	s.totalIPs = totalIPs
	s.log.Infof("total ip of dhcp server: %s", totalIPs)

	return configFile, nil
}
