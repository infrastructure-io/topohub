package dhcpserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// generateDnsmasqConfig generates the dnsmasq configuration file
func (s *dhcpServer) generateDnsmasqConfig() error {

	s.log.Infof("generating config")

	// 读取模板文件
	tmpl, err := template.ParseFiles(s.configTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	templateContent, err := os.ReadFile(s.configTemplatePath)
	if err != nil {
		s.log.Errorf("failed to read template file: %+v", err)
		return err
	}
	s.log.Debugf("read template file content: \n%s", string(templateContent))

	// 准备目录
	configFile := s.configPath
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// 准备接口名称
	var interfaceName string
	if s.subnet.Spec.Interface.VlanID != nil && *s.subnet.Spec.Interface.VlanID > 0 {
		interfaceName = fmt.Sprintf(vlanInterfaceFormat, s.subnet.Spec.Interface.Interface, *s.subnet.Spec.Interface.VlanID)
	} else {
		interfaceName = s.subnet.Spec.Interface.Interface
	}

	ipRange := strings.Split(s.subnet.Spec.IPv4Subnet.IPRange, ",")
	for k := range ipRange {
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
		PxeEfiInTftpServerDir    string
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
		TftpServerDir:            s.config.StoragePathTftp,
		PxeEfiInTftpServerDir:    s.config.StoragePathTftpAbsoluteDirForPxeEfi,
		HostIpBindingsConfigPath: s.HostIpBindingsConfigPath,
	}

	// 删除已存在的配置文件
	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing config file: %v", err)
	}
	f, err := os.Create(configFile)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			s.log.Infof("config file already exists: %s", configFile)
		} else {
			return fmt.Errorf("failed to create config file: %v", err)
		}
	}
	s.log.Infof("Generated dnsmasq config file: %s", configFile)
	defer f.Close()

	s.log.Debugf("Generated dnsmasq config: %+v", data)

	// 写入配置
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	// make sure the binding config file exists
	if _, err := os.ReadFile(s.HostIpBindingsConfigPath); err != nil && os.IsNotExist(err) && s.subnet.Spec.Feature.EnableBindDhcpIP {
		// 如果文件不存在，创建文件
		if err := os.MkdirAll(filepath.Dir(s.HostIpBindingsConfigPath), 0755); err != nil {
			s.log.Panicf("failed to create directory for bindings file: %v", err)
		}
		if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(""), 0644); err != nil {
			s.log.Panicf("failed to create bindings file: %v", err)
		}
		s.log.Infof("created new bindings file: %s", s.HostIpBindingsConfigPath)
	}
	// update the binding config
	_, err = s.processLeaseAndUpdateBindings(true)
	if err != nil {
		return fmt.Errorf("failed to write binding config: %v", err)
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		s.log.Errorf("failed to read content file: %+v", err)
		return err
	}
	s.log.Debugf("read config file: \n%s", string(content))

	return nil
}

// processLeaseFile reads and processes the lease file
func (s *dhcpServer) processLeaseAndUpdateBindings(ignoreLeaseExistenceError bool) (needReload bool, finalErr error) {
	leaseFile := s.leasePath
	needReload = false

	// 读取租约文件
	content, err := os.ReadFile(leaseFile)
	if err != nil {
		if os.IsNotExist(err) && ignoreLeaseExistenceError {
			s.log.Debugf("ignore lease file: %s", leaseFile)
			return needReload, nil
		}
		return needReload, fmt.Errorf("failed to read lease file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	currentClients := make(map[string]*DhcpClientInfo)

	s.lockData.Lock()
	defer s.lockData.Unlock()
	previousClients := s.currentClients

	// 处理每一行租约记录
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			s.log.Warnf("invalid lease line: %s", line)
			continue
		}

		// 解析租约信息
		expireTimestamp, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			s.log.Warnf("failed to parse lease expiration time: %v", err)
			continue
		}
		expireTime := time.Unix(expireTimestamp, 0)

		clusterName := ""
		if s.subnet.Spec.Feature.EnableSyncEndpoint != nil && s.subnet.Spec.Feature.EnableSyncEndpoint.DefaultClusterName != nil {
			clusterName = *s.subnet.Spec.Feature.EnableSyncEndpoint.DefaultClusterName
		}

		clientInfo := &DhcpClientInfo{
			MAC:            fields[1],
			IP:             fields[2],
			Hostname:       fields[3],
			Active:         true,
			DhcpExpireTime: expireTime,
			Subnet:         s.subnet.Spec.IPv4Subnet.Subnet,
			SubnetName:     s.subnet.Name,
			ClusterName:    clusterName,
		}
		currentClients[clientInfo.IP] = clientInfo

		// 检查是否为新增客户端
		if s.subnet.Spec.Feature.EnableBindDhcpIP {
			if data, exists := previousClients[clientInfo.IP]; !exists {
				s.addedDhcpClient <- *clientInfo
				s.log.Infof("send event to add dhcp client: %s, %s", clientInfo.MAC, clientInfo.IP)
				// bind new client to config and reload the server
				needReload = true
			} else {
				if data.MAC != clientInfo.MAC || data.Hostname != clientInfo.Hostname {
					s.addedDhcpClient <- *clientInfo
					s.log.Infof("send event to update dhcp client, old mac=%s, new mac=%s, old hostname=%s, new hostname=%s, ip=%s", data.MAC, clientInfo.MAC, data.Hostname, clientInfo.Hostname, clientInfo.IP)
					// bind new client to conf
					needReload = true
				} else {
					if clientInfo.DhcpExpireTime.Equal(previousClients[clientInfo.IP].DhcpExpireTime) {
						s.addedDhcpClient <- *clientInfo
						s.log.Infof("send event to update the ExpireTime for dhcp client: %s, %s", clientInfo.MAC, clientInfo.IP)
					}
				}
			}
		}
	}

	// 检查删除的客户端
	for _, client := range previousClients {
		if _, exists := currentClients[client.IP]; !exists {
			client.Active = false
			if s.subnet.Spec.Feature.EnableBindDhcpIP {
				s.deletedDhcpClient <- *client
				s.log.Infof("send event to delete dhcp client: %s, %s", client.MAC, client.IP)
			}
		}
	}

	// 更新客户端缓存和统计信息
	s.currentClients = currentClients

	if s.subnet.Spec.Feature.EnableBindDhcpIP && needReload {
		// make sure new binding config exists in the config file
		s.log.Infof("EnableBindDhcpIP is true, generate bindings file: %s", s.HostIpBindingsConfigPath)
		newbindings := make(map[string]string)
		for _, client := range s.currentClients {
			newbindings[client.IP] = client.MAC
		}
		if err := s.UpdateDhcpBindings(newbindings, nil); err != nil {
			s.log.Errorf("failed to add dhcp bindings: %v", err)
			return false, err
		}
	} else {
		// just update the newest binding information
		if err := s.UpdateDhcpBindings(nil, nil); err != nil {
			s.log.Errorf("failed to update dhcp bindings: %v", err)
		}

	}

	return needReload, nil
}

// UpdateDhcpBindings updates the dhcp-host configuration file by:
// 1. For ipMacMapAdded: if IP exists, update its MAC; if IP doesn't exist, add new binding
// 2. For ipMacMapDeleted: delete binding only if both IP and MAC match exactly
func (s *dhcpServer) UpdateDhcpBindings(added, deleted map[string]*DhcpClientInfo) error {
	s.log.Debugf("processing dhcp bindings, added: %+v, deleted: %+v", added, deleted)

	s.lockConfigUpdate.Lock()
	defer s.lockConfigUpdate.Unlock()

	bindClients := map[string]*DhcpClientInfo{}

	// 读取现有的配置文件
	content, err := os.ReadFile(s.HostIpBindingsConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// make sure the bindings file exists
			if s.subnet.Spec.Feature.EnableBindDhcpIP {
				// 如果文件不存在，创建文件
				if err := os.MkdirAll(filepath.Dir(s.HostIpBindingsConfigPath), 0755); err != nil {
					s.log.Panicf("failed to create directory for bindings file: %v", err)
				}
				if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(""), 0644); err != nil {
					s.log.Panicf("failed to create bindings file: %v", err)
				}
				s.log.Infof("created new bindings file: %s", s.HostIpBindingsConfigPath)
			} else {
				s.log.Debugf("bindings file does not exist: %s, skip to process dhcp bindings", s.HostIpBindingsConfigPath)
				return nil
			}
		} else {
			return fmt.Errorf("failed to read bindings file, err: %v", err)

		}
	}

	var finalLines []string
	processedIPs := make(map[string]bool)
	lines := strings.Split(string(content), "\n")

	// 遍历每一行，处理现有的绑定
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否是 dhcp-host 配置行
		if strings.HasPrefix(line, "dhcp-host=") {
			// 解析 MAC 和 IP
			parts := strings.Split(line, "=")[1]
			fields := strings.Split(parts, ",")
			if len(fields) < 2 {
				s.log.Warnf("invalid dhcp-host line format: %s", line)
				continue
			}

			mac := fields[0]
			ip := fields[1]

			// 检查是否需要删除这行配置
			if expectedMac, exists := deleted[ip]; exists && expectedMac == mac {
				s.log.Infof("removing dhcp-host binding for IP %s, MAC %s", ip, mac)
				continue
			}

			// 检查是否需要更新 MAC
			if item, exists := added[ip]; exists {
				s.log.Infof("updating dhcp-host binding for IP %s: old MAC %s -> new MAC %s", ip, mac, newMac)
				finalLines = append(finalLines, fmt.Sprintf("dhcp-host=%s,%s", newMac, ip))
				processedIPs[ip] = true
				bindClients[ip] = &DhcpClientInfo{
					MAC: newMac,
					IP:  ip,
				}
				continue
			}

			// 保持原有配置不变
			finalLines = append(finalLines, line)
			processedIPs[ip] = true
			bindClients[ip] = &DhcpClientInfo{
				MAC: mac,
				IP:  ip,
			}
		} else {
			// 非 dhcp-host 配置行保持不变
			finalLines = append(finalLines, line)
		}
	}

	// 添加新的绑定（仅处理尚未处理的IP）
	for ip, mac := range ipMacMapAdded {
		if !processedIPs[ip] {
			s.log.Infof("adding new dhcp-host binding for IP %s, MAC %s", ip, mac)
			finalLines = append(finalLines, fmt.Sprintf("dhcp-host=%s,%s", mac, ip))
			bindClients[ip] = &DhcpClientInfo{
				MAC: mac,
				IP:  ip,
			}
		}
	}

	// 写入更新后的配置
	if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(strings.Join(finalLines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write bindings file: %v", err)
	}

	// update the bind clients
	s.bindClients = bindClients

	return nil
}
