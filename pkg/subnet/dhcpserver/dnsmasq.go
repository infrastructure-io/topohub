package dhcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// startDnsmasq starts the dnsmasq process
func (s *dhcpServer) startDnsmasq() error {

	if err := s.setupInterface(); err != nil {
		return fmt.Errorf("failed to setup interface: %v", err)
	}

	configFilePath, err := s.generateDnsmasqConfig(s.currentClients)
	if err != nil {
		return fmt.Errorf("failed to generate dnsmasq config: %v", err)
	}
	s.log.Infof("dns config file %s", configFilePath)

	// 创建 context 用于进程管理
	ctx, cancel := context.WithCancel(context.Background())
	s.cmdCancel = cancel

	// 启动 dnsmasq
	cmd := exec.Command("dnsmasq", "-C", configFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnsmasq: %v", err)
	}

	s.cmd = cmd

	go func() {
		// cancel the ctx
		defer cancel()
		if err := cmd.Wait(); err != nil {
			if ctx.Err() == nil {
				s.log.Errorf("dnsmasq process exited unexpectedly: %v", err)
			}
		}
	}()

	// 等待进程启动
	time.Sleep(time.Second)

	// 检查进程是否正常运行
	if cmd.Process == nil {
		return fmt.Errorf("dnsmasq process failed to start")
	}

	return nil
}

// UpdateService updates the subnet configuration and restarts the DHCP server
func (s *dhcpServer) UpdateService(subnet topohubv1beta1.Subnet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新 subnet
	s.subnet = &subnet
	// 重启 DHCP 服务
	s.restartCh <- struct{}{}

	return nil
}

// processLeaseFile reads and processes the lease file
func (s *dhcpServer) processLeaseFile(leaseFile string) error {
	// 读取租约文件
	content, err := os.ReadFile(leaseFile)
	if err != nil {
		return fmt.Errorf("failed to read lease file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	currentClients := make(map[string]*DhcpClientInfo)

	s.mu.Lock()
	previousClients := s.currentClients
	s.mu.Unlock()

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
	s.mu.Lock()
	s.currentClients = currentClients
	s.mu.Unlock()

	// 更新 Subnet 状态
	updated := s.subnet.DeepCopy()
	if updated.Status.DhcpStatus == nil {
		updated.Status.DhcpStatus = &topohubv1beta1.DhcpStatusSpec{}
	}
	updated.Status.DhcpStatus.DhcpIpTotalAmount = s.totalIPs
	updated.Status.DhcpStatus.DhcpIpAssignAmount = uint64(len(currentClients))
	updated.Status.DhcpStatus.DhcpIpAvailableAmount = s.totalIPs - uint64(len(currentClients))

	// 发送状态更新
	s.statusUpdateCh <- updated

	return nil
}

// monitor monitors the lease file and updates status
func (s *dhcpServer) monitor() {

	// 添加 lease 文件监控
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Errorf("Failed to create lease file watcher: %v", err)
		return
	}
	defer watcher.Close()
	if err := watcher.Add(filepath.Dir(s.leasePath)); err != nil {
		s.log.Errorf("Failed to watch lease file: %v", err)
		return
	}

	// watch the process at an interval
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// 开始监控
	for {
		needRestart := false
		needReload := false
		select {
		case <-s.stopCh:
			s.log.Errorf("subnet monitor is exiting")
			return

		// lease file event
		case event := <-watcher.Events:
			if event.Name == s.leasePath && (event.Op&fsnotify.Write == fsnotify.Write) {
				if err := s.processLeaseFile(s.leasePath); err != nil {
					s.log.Errorf("Failed to process lease file: %v", err)
				}
			}
			if s.subnet.Spec.Feature.EnableBindDhcpIP {
				needReload = true
			}

		// watch error of lease file
		case err := <-watcher.Errors:
			s.log.Errorf("Lease file watcher error: %v", err)

		// subnet changes
		case <-s.restartCh:
			s.log.Infof("dhcp server reload after receiving a subnet updating event")
			needReload = true

		// check the process
		case <-ticker.C:
			isDead := s.cmd == nil || s.cmd.Process == nil
			if !isDead {
				if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
					log.Logger.Errorf("DHCP server process check failed: %v", err)
					needRestart = true
				}
			} else {
				needRestart = true
				s.log.Infof("dhcp server is dead, restart it")
			}
		}

		if needRestart || needReload {

			configFilePath, err := s.generateDnsmasqConfig(s.currentClients)
			if err != nil {
				s.log.Errorf("Failed to update dnsmasq config: %v", err)
				continue
			}

			if needReload {
				s.log.Infof("reload dhcp server")
				// 重新加载 dnsmasq 配置
				if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
					s.log.Errorf("failed to reload dnsmasq: %v", err)
					continue
				}
				s.log.Infof("Reloaded dnsmasq config: %s", configFilePath)
			} else {
				s.log.Infof("restarting dhcp server")
				s.startDnsmasq()
			}
		}
	}
}
