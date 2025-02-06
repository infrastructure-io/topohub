package dhcpserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/infrastructure-io/topohub/pkg/tools"
)

// generateDnsmasqConfig generates the dnsmasq configuration file
func (s *dhcpServer) generateDnsmasqConfig() (string, error) {

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

	ipRange := strings.Split(s.subnet.Spec.IPv4Subnet.IPRange, ",")
	for k, _ := range ipRange {
		ipRange[k] = strings.ReplaceAll(ipRange[k], "-", ",")
	}

	data := struct {
		Interface                string
		IPRanges                 []string
		Gateway                  *string
		DNS                      *string
		LeaseFile                string
		LogFile                  string
		EnablePxe                bool
		EnableZtp                bool
		EnableBindDhcpIP         bool
		Name                     string
		SelfIP                   string
		TftpServerDir            string
		HostIpBindingsConfigPath string
	}{
		Interface:                interfaceName,
		IPRanges:                 ipRange,
		Gateway:                  s.subnet.Spec.IPv4Subnet.Gateway,
		DNS:                      s.subnet.Spec.IPv4Subnet.Dns,
		LeaseFile:                s.leasePath,
		LogFile:                  s.logPath,
		EnablePxe:                s.subnet.Spec.Feature.EnablePxe,
		EnableZtp:                s.subnet.Spec.Feature.EnableZtp,
		EnableBindDhcpIP:         s.subnet.Spec.Feature.EnableBindDhcpIP,
		Name:                     s.subnet.Name,
		SelfIP:                   strings.Split(s.subnet.Spec.Interface.IPv4, "/")[0],
		TftpServerDir:            s.config.StoragePathSftp,
		HostIpBindingsConfigPath: s.HostIpBindingsConfigPath,
	}

	// 删除已存在的配置文件
	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove existing config file: %v", err)
	}
	f, err := os.Create(configFile)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			s.log.Infof("config file already exists: %s", configFile)
		} else {
			return "", fmt.Errorf("failed to create config file: %v", err)
		}
	}
	s.log.Infof("Generated dnsmasq config file: %s", configFile)
	defer f.Close()

	s.log.Debugf("Generated dnsmasq config: %+v", data)

	// 写入配置
	if err := tmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("failed to write config: %v", err)
	}

	// update the binding config
	err = s.processLeaseAndUpdateBindings(true)
	if err != nil {
		return "", fmt.Errorf("failed to write binding config: %v", err)
	}

	// 统计 IP 使用情况
	total, err := tools.CountIPsInRange(s.subnet.Spec.IPv4Subnet.IPRange)
	if err != nil {
		s.log.Errorf("failed to count ips in range: %+v", err)
		total = 0
	}
	s.totalIPs = total
	s.log.Infof("total ip of dhcp server: %v", total)

	content, err := os.ReadFile(configFile)
	if err != nil {
		s.log.Errorf("failed to read content file: %+v", err)
		return "", err
	}
	s.log.Debugf("read config file: \n%s", string(content))

	return configFile, nil
}

// processLeaseFile reads and processes the lease file
func (s *dhcpServer) processLeaseAndUpdateBindings(ignoreLease bool) error {
	leaseFile := s.leasePath

	existingContent := []byte("")
	var err error
	if s.subnet.Spec.Feature.EnableBindDhcpIP {
		// 读取现有的绑定配置
		existingContent, err = os.ReadFile(s.HostIpBindingsConfigPath)
		if err != nil && os.IsNotExist(err) {
			// 如果文件不存在，创建文件
			if err := os.MkdirAll(filepath.Dir(s.HostIpBindingsConfigPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for bindings file: %v", err)
			}
			if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(""), 0644); err != nil {
				return fmt.Errorf("failed to create bindings file: %v", err)
			}
			s.log.Infof("created new bindings file: %s", s.HostIpBindingsConfigPath)
		}
	}

	// 读取租约文件
	content, err := os.ReadFile(leaseFile)
	if err != nil {
		if os.IsNotExist(err) && ignoreLease {
			s.log.Debugf("ignore lease file: %s", leaseFile)
			return nil
		}
		return fmt.Errorf("failed to read lease file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	currentClients := make(map[string]*DhcpClientInfo)

	s.mu.Lock()
	defer s.mu.Unlock()
	previousClients := s.currentClients

	// 处理每一行租约记录
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// 解析租约信息
		clientInfo := &DhcpClientInfo{
			MAC:       fields[1],
			IP:        fields[2],
			Active:    true,
			StartTime: fields[0],
			Subnet:    s.subnet.Spec.IPv4Subnet.Subnet,
		}

		currentClients[clientInfo.MAC] = clientInfo

		// 检查是否为新增客户端
		if _, exists := previousClients[clientInfo.MAC]; !exists {
			s.addedDhcpClient <- *clientInfo
		}
	}

	// 检查删除的客户端
	for mac, client := range previousClients {
		if _, exists := currentClients[mac]; !exists {
			client.Active = false
			s.deletedDhcpClient <- *client
		}
	}

	// 更新客户端缓存和统计信息
	s.currentClients = currentClients

	if s.subnet.Spec.Feature.EnableBindDhcpIP {
		s.log.Infof("EnableBindDhcpIP is true, generate bindings file: %s", s.HostIpBindingsConfigPath)
		var existingLines []string

		// 解析现有的绑定记录，使用 IP 作为键
		existingBindings := make(map[string]string) // IP -> 完整绑定记录
		if len(existingContent) > 0 {
			lines := strings.Split(string(existingContent), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "dhcp-host=") {
					parts := strings.Split(strings.TrimPrefix(line, "dhcp-host="), ",")
					if len(parts) == 2 {
						ip := parts[1]
						existingBindings[ip] = line
					}
				}
				existingLines = append(existingLines, line)
				s.log.Debugf("existing binding: %s", line)
			}
		}

		// 准备新的绑定记录
		newBindings := make(map[string]string) // IP -> 新的绑定记录
		for _, client := range currentClients {
			newBindings[client.IP] = fmt.Sprintf("dhcp-host=%s,%s", client.MAC, client.IP)
		}

		// 创建新的配置文件内容
		var finalLines []string
		for _, line := range existingLines {
			if strings.HasPrefix(line, "dhcp-host=") {
				parts := strings.Split(strings.TrimPrefix(line, "dhcp-host="), ",")
				if len(parts) == 2 {
					ip := parts[1]
					if newBinding, exists := newBindings[ip]; exists {
						// 使用新的绑定记录替换旧的
						s.log.Infof("using new binding for IP %s, old mac %s, new mac %s", ip, parts[0], newBinding)
						finalLines = append(finalLines, newBinding)
						delete(newBindings, ip)
					} else {
						// 保留旧的绑定记录
						// keep existing line
						finalLines = append(finalLines, line)
					}
				}
			} else {
				// keep existing line
				finalLines = append(finalLines, line)
			}
		}

		// 添加剩余的新绑定记录
		for _, binding := range newBindings {
			s.log.Infof("adding new binding: %s", binding)
			finalLines = append(finalLines, binding)
		}

		// 写入更新后的配置
		if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(strings.Join(finalLines, "\n")+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write bindings file: %v", err)
		}
	}

	return nil
}
