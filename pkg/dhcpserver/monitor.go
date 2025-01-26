package dhcpserver

import (
	"fmt"

	"github.com/infrastructure-io/topohub/pkg/log"

	"syscall"
	"time"

	hoststatusdata "github.com/infrastructure-io/topohub/pkg/agent/hoststatus/data"

	"github.com/infrastructure-io/topohub/pkg/dhcpserver/types"
)

// monitor periodically checks server health and updates statistics.
// This routine runs in a separate goroutine and continues until
// signaled to stop via stopChan.
func (s *dhcpServer) monitor() {
	ticker := time.NewTicker(MonitorInterval * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 周期确认 dhcp lease 中是否有 新的 ip 分配变化，从而上报 到 hoststatus
		if err := s.updateStats(); err != nil {
			log.Logger.Debugf("Failed to update stats during monitoring: %v", err)
		} else {
			log.Logger.Debugf("Successfully updated DHCP server stats - Available IPs: %d, Used: %d, Usage: %.2f%%",
				s.stats.AvailableIPs, s.stats.UsedIPs, s.stats.UsagePercentage)
		}

		// 周期确认 dhcp server 是否存活，是否要重启 dhcp server
		// 周期确认 是否要更新 dhcp 的 固定 ip，是否要重启 dhcp server
		needRestart := s.cmd == nil || s.cmd.Process == nil
		if !needRestart {
			// Process exists, check if it's still running
			if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
				log.Logger.Debugf("DHCP server process check failed: %v", err)
				needRestart = true
			} else {
				log.Logger.Debug("DHCP server process health check passed")

				// 周期确认 是否要更新 dhcp 的 固定 ip，是否要重启 dhcp server
				if s.checkHostStatusForFixedIPs() {
					log.Logger.Infof("Bond IP for host status should be updated, restart dhcp server")
					needRestart = true
				}
			}
		}
		if needRestart {
			// Print last 50 lines of log file before restart
			if err := s.printDhcpLogTail(); err != nil {
				log.Logger.Errorf("Failed to print DHCP log tail: %v", err)
			}
			if err := s.Stop(); err != nil {
				log.Logger.Warnf("Failed to stop DHCP server: %v", err)
			}
			if err := s.Start(); err != nil {
				log.Logger.Errorf("Failed to restart DHCP server: %v", err)
			}
		}
	}
}

// updateStats calculates current IP allocation statistics.
// It counts the number of active leases and compares with total
// available IPs to determine usage statistics.
//
// Returns an error if statistics cannot be calculated.
func (s *dhcpServer) updateStats() error {

	//log.Logger.Debug("Updating DHCP server statistics")
	s.mutex.Lock()
	defer s.mutex.Unlock()

	clients, err := s.GetClientInfo()
	if err != nil {
		log.Logger.Errorf("Failed to get client info during stats update: %v", err)
		return fmt.Errorf("failed to get client info: %v", err)
	}

	// Compare with previous clients to detect changes
	newClients := make(map[string]types.ClientInfo)
	oldClients := make(map[string]types.ClientInfo)

	for _, client := range clients {
		newClients[client.IP] = client
	}
	for _, client := range s.previousClients {
		oldClients[client.IP] = client
	}

	if len(newClients) != len(oldClients) {
		log.Logger.Debugf("get GetClientInfo: %+v", clients)
		log.Logger.Infof("Checking for client changes - Current clients: %d, Previous clients: %d",
			len(newClients), len(oldClients))
	}

	// Check for new clients and MAC changes
	for ip, newClient := range newClients {
		oldClient, exists := oldClients[ip]
		if !exists {
			log.Logger.Infof("New DHCP client detected - IP: %s, MAC: %s", ip, newClient.MAC)
			// New IP allocation
			s.addChan <- newClient
		} else if oldClient.MAC != newClient.MAC {
			// Same IP but MAC changed, send delete followed by add
			log.Logger.Infof("DHCP client changed the mac - IP: %s, MAC: %s", ip, newClient.MAC)
			s.addChan <- newClient
		}
	}

	// Check for removed clients
	for ip, oldClient := range oldClients {
		if _, exists := newClients[ip]; !exists {
			log.Logger.Infof("Deleted DHCP client detected - IP: %s, MAC: %s", ip, oldClient.MAC)
			s.deleteChan <- oldClient
		}
	}

	s.previousClients = clients
	s.stats.UsedIPs = len(clients)
	s.stats.AvailableIPs = s.totalIPs - s.stats.UsedIPs
	if s.totalIPs > 0 {
		s.stats.UsagePercentage = float64(s.stats.UsedIPs) / float64(s.totalIPs) * 100
	}

	log.Logger.Debugf("DHCP server statistics updated - Available IPs: %d, Used: %d, Usage: %.2f%%",
		s.stats.AvailableIPs, s.stats.UsedIPs, s.stats.UsagePercentage)

	return nil
}

func (s *dhcpServer) checkHostStatusForFixedIPs() bool {

	if !s.config.EnableBindDhcpIP && !s.config.EnableBindStaticIP {
		return false
	}

	s.lastBoundIPLock.Lock()
	defer s.lastBoundIPLock.Unlock()

	fixedIPs := map[string]string{}
	if s.config.EnableBindStaticIP {
		dhcpclientList := hoststatusdata.HostCacheDatabase.GetDhcpClientInfo()
		for _, client := range dhcpclientList {
			fixedIPs[client.Info.IpAddr] = client.Info.Mac
		}
	}
	if s.config.EnableBindStaticIP {
		staticclientList := hoststatusdata.HostCacheDatabase.GetStaticClientInfo()
		for _, client := range staticclientList {
			fixedIPs[client.Info.IpAddr] = virtualMac
		}
	}

	// 检查是否有任何变化
	hasChanges := false

	// 检查新增或修改的 IP
	for ip, newMAC := range fixedIPs {
		oldMAC, exists := s.lastBoundIPList[ip]
		if !exists {
			log.Logger.Infof("New IP binding found: ip=%s, mac=%s", ip, newMAC)
			hasChanges = true
		} else if oldMAC != newMAC {
			log.Logger.Infof("MAC changed for IP %s: old_mac=%s, new_mac=%s", ip, oldMAC, newMAC)
			hasChanges = true
		}
	}

	// 检查删除的 IP
	for ip, oldMAC := range s.lastBoundIPList {
		if _, exists := fixedIPs[ip]; !exists {
			log.Logger.Infof("IP binding removed: ip=%s, old_mac=%s", ip, oldMAC)
			hasChanges = true
		}
	}

	// 比较新旧 fixedIPs 的差异
	if len(fixedIPs) != len(s.lastBoundIPList) {
		log.Logger.Infof("Fixed IPs count changed: old=%d, new=%d", len(s.lastBoundIPList), len(fixedIPs))
	}

	return hasChanges
}
