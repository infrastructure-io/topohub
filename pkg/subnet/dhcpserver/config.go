package dhcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/infrastructure-io/topohub/pkg/tools"
)

// generateDnsmasqConfig generates the dnsmasq configuration file
func (s *dhcpServer) generateDnsmasqConfig(clients map[string]*DhcpClientInfo) (string, error) {

	s.log.Infof("generating config")

	// 读取模板文件
	tmpl, err := template.ParseFiles(s.configTemplatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	templateContent, err := os.ReadFile(s.configTemplatePath)
	if err != nil {
		s.log.Errorf("failed to read template file: %+v", err)
		return "", err
	}
	s.log.Debugf("read template file content: \n%s", string(templateContent))

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
		Name             string
		SelfIP           string
		TftpServerDir    string
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
		Name:             s.subnet.Name,
		SelfIP:           strings.Split(s.subnet.Spec.Interface.IPv4, "/")[0],
		TftpServerDir:    s.config.StoragePathSftp,
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
	total, err := tools.CountIPsInRange(s.subnet.Spec.IPv4Subnet.IPRange)
	if err != nil {
		s.log.Errorf("failed to count ips in range: %+v", err)
		total = 0
	}
	s.totalIPs = total
	s.log.Infof("total ip of dhcp server: %s", total)

	content, err := os.ReadFile(configFile)
	if err != nil {
		s.log.Errorf("failed to read content file: %+v", err)
		return "", err
	}
	s.log.Debugf("read config file: \n%s", string(content))

	return configFile, nil
}
