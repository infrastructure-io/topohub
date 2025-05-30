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

	//-------------------- prepare the binding config -------------
	// make sure the binding config file exists
	if _, err := os.ReadFile(s.HostIpBindingsConfigPath); err != nil && os.IsNotExist(err) {
		// 如果文件不存在，创建文件
		if err := os.MkdirAll(filepath.Dir(s.HostIpBindingsConfigPath), 0755); err != nil {
			s.log.Panicf("failed to create directory for bindings file: %v", err)
		}
		if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(""), 0644); err != nil {
			s.log.Panicf("failed to create bindings file: %v", err)
		}
		s.log.Infof("created new bindings file: %s", s.HostIpBindingsConfigPath)
	}
	// update the lease
	if _, err := s.processDhcpLease(true); err != nil {
		return fmt.Errorf("failed to process lease file: %v", err)
	}

	// finally update the binding config
	if err := s.UpdateDhcpBindings(); err != nil {
		s.log.Errorf("failed to add dhcp bindings: %v", err)
		return err
	}

	//-------------------- generate the config file --------------------
	content, err := os.ReadFile(configFile)
	if err != nil {
		s.log.Errorf("failed to read content file: %+v", err)
		return err
	}
	s.log.Debugf("read config file: \n%s", string(content))

	return nil
}

// processLeaseFile reads and processes the lease file
// 1. 获取新的 client， 通知 RedfishStatus 模块
func (s *dhcpServer) processDhcpLease(ignoreLeaseExistenceError bool) (clientChangedFlag bool, finalErr error) {
	leaseFile := s.leasePath
	clientChangedFlag = false

	// 读取租约文件
	content, err := os.ReadFile(leaseFile)
	if err != nil {
		if os.IsNotExist(err) && ignoreLeaseExistenceError {
			s.log.Debugf("ignore lease file: %s", leaseFile)
			return false, nil
		}
		return false, fmt.Errorf("failed to read lease file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	currentLeaseClients := make(map[string]*DhcpClientInfo)

	s.lockData.Lock()
	defer s.lockData.Unlock()
	previousClients := s.currentLeaseClients

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
		if s.subnet.Spec.Feature.SyncRedfishstatus.DefaultClusterName != nil {
			clusterName = *s.subnet.Spec.Feature.SyncRedfishstatus.DefaultClusterName
		}

		enableBindIP := false
		if s.subnet.Spec.Feature.SyncRedfishstatus.Enabled && s.subnet.Spec.Feature.SyncRedfishstatus.EnableBindDhcpIP {
			enableBindIP = true
		}

		clientInfo := &DhcpClientInfo{
			MAC:                          fields[1],
			IP:                           fields[2],
			Hostname:                     fields[3],
			Active:                       true,
			DhcpExpireTime:               expireTime,
			Subnet:                       s.subnet.Spec.IPv4Subnet.Subnet,
			SubnetName:                   s.subnet.Name,
			ClusterName:                  clusterName,
			EnableBindIpForRedfishstatus: &enableBindIP,
		}
		currentLeaseClients[clientInfo.IP] = clientInfo

		// redfishstatus 进行 crd 实例同步

		if data, exists := previousClients[clientInfo.IP]; !exists {
			if s.subnet.Spec.Feature.SyncRedfishstatus.Enabled {
				// redfishstatus 进行 crd 实例同步
				s.addedDhcpClientForRedfishStatus <- *clientInfo
				s.log.Infof("send event to add dhcp client: %s, %s", clientInfo.MAC, clientInfo.IP)
			}
			clientChangedFlag = true

		} else {
			if data.MAC != clientInfo.MAC || data.Hostname != clientInfo.Hostname {
				if s.subnet.Spec.Feature.SyncRedfishstatus.Enabled {
					// redfishstatus 进行 crd 实例同步
					s.addedDhcpClientForRedfishStatus <- *clientInfo
					s.log.Infof("send event to update dhcp client, old mac=%s, new mac=%s, old hostname=%s, new hostname=%s, ip=%s", data.MAC, clientInfo.MAC, data.Hostname, clientInfo.Hostname, clientInfo.IP)
				}
				clientChangedFlag = true
			} else if !clientInfo.DhcpExpireTime.Equal(previousClients[clientInfo.IP].DhcpExpireTime) {
				if s.subnet.Spec.Feature.SyncRedfishstatus.Enabled {
					s.addedDhcpClientForRedfishStatus <- *clientInfo
					s.log.Infof("send event to update dhcp client for its DhcpExpireTime: %s, %s, oldDhcpExpireTime=%s, newDhcpExpireTime=%s", clientInfo.MAC, clientInfo.IP, previousClients[clientInfo.IP].DhcpExpireTime, clientInfo.DhcpExpireTime)
				}
			}
		}
	}

	// 检查删除的客户端
	for _, client := range previousClients {
		if _, exists := currentLeaseClients[client.IP]; !exists {
			client.Active = false
			if s.subnet.Spec.Feature.SyncRedfishstatus.Enabled {
				s.deletedDhcpClientForRedfishStatus <- *client
				s.log.Infof("send event to delete dhcp client: %s, %s", client.MAC, client.IP)
				// 对于删除的 dhcp 客户端，不进行 ip 解绑，确保安全
			}
		}
	}

	// 更新客户端缓存和统计信息
	s.currentLeaseClients = currentLeaseClients

	return clientChangedFlag, nil
}

// UpdateDhcpBindings updates the dhcp-host configuration file by:
// 1. For ipMacMapAdded: if IP exists, update its MAC; if IP doesn't exist, add new binding
// 2. For ipMacMapDeleted: delete binding only if both IP and MAC match exactly
func (s *dhcpServer) UpdateDhcpBindings() error {

	// 读取现有的配置文件
	_, err := os.ReadFile(s.HostIpBindingsConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 如果文件不存在，创建文件
			if err := os.MkdirAll(filepath.Dir(s.HostIpBindingsConfigPath), 0755); err != nil {
				s.log.Panicf("failed to create directory for bindings file: %v", err)
			}
			if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(""), 0644); err != nil {
				s.log.Panicf("failed to create bindings file: %v", err)
			}
			s.log.Infof("created new bindings file: %s", s.HostIpBindingsConfigPath)
		} else {
			return fmt.Errorf("failed to read bindings file, err: %v", err)

		}
	}

	s.lockConfigUpdate.Lock()
	defer s.lockConfigUpdate.Unlock()

	s.log.Debugf("processing dhcp bindings: %+v ", s.currentManualBindingClients)

	var finalLines []string
	for ip, item := range s.currentManualBindingClients {
		s.log.Debugf("adding new dhcp-host binding for IP %s, MAC %s", ip, item.MAC)
		if len(item.Hostname) > 0 {
			finalLines = append(finalLines, "# hostname "+item.Hostname)
		}
		line := fmt.Sprintf("dhcp-host=%s,%s", item.MAC, ip)
		finalLines = append(finalLines, line)
	}

	// 写入更新后的配置
	if err := os.WriteFile(s.HostIpBindingsConfigPath, []byte(strings.Join(finalLines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write bindings file: %v", err)
	}

	return nil
}
