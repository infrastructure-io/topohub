package dhcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// generateDnsmasqConfig generates the dnsmasq configuration file
func (s *dhcpServer) generateDnsmasqConfig(clients map[string]*DhcpClientInfo) (string, error) {
	// 读取模板文件
	tmpl, err := template.ParseFiles(filepath.Join(s.config.DhcpConfigTemplatePath, "dnsmasq.conf.tmpl"))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	// 准备目录
	configFile := filepath.Join(s.config.StoragePathDhcpConfig, fmt.Sprintf("dnsmasq-%s.conf", s.subnet.Name))
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
		Interface     string
		IPRanges     []string
		Gateway      *string
		DNS          *string
		LeaseFile    string
		LogFile      string
		EnablePxe    bool
		EnableZtp    bool
		DhcpHostBindings []string
	}{
		Interface:     interfaceName,
		IPRanges:     strings.Split(s.subnet.Spec.IPv4Subnet.IPRange, ","),
		Gateway:      s.subnet.Spec.IPv4Subnet.Gateway,
		DNS:          s.subnet.Spec.IPv4Subnet.Dns,
		LeaseFile:    filepath.Join(s.config.StoragePathDhcpLease, fmt.Sprintf("dnsmasq-%s.leases", s.subnet.Name)),
		LogFile:      filepath.Join(s.config.StoragePathDhcpLog, fmt.Sprintf("dnsmasq-%s.log", s.subnet.Name)),
		EnablePxe:    s.subnet.Spec.Feature.EnablePxe,
		EnableZtp:    s.subnet.Spec.Feature.EnableZtp,
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

	return configFile, nil
}
