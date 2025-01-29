package dhcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"syscall"

	"github.com/fsnotify/fsnotify"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// startDnsmasq starts the dnsmasq process
func (s *dhcpServer) startDnsmasq() error {
	configFilePath, err := s.generateDnsmasqConfig(s.currentClients)
	if err != nil {
		return fmt.Errorf("failed to generate dnsmasq config: %v", err)
	}

	// 创建 context 用于进程管理
	ctx, cancel := context.WithCancel(context.Background())
	s.cmdCancel = cancel

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

	// 启动 dnsmasq
	cmd := exec.Command("dnsmasq", "-C", configFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnsmasq: %v", err)
	}

	s.cmd = cmd

	// 启动进程监控
	go func() {
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
	s.stats.TotalIPs = s.totalIPs
	s.stats.UsedIPs = len(currentClients)
	s.stats.AvailableIPs = s.totalIPs - len(currentClients)
	s.mu.Unlock()

	// 更新 Subnet 状态
	updated := s.subnet.DeepCopy()
	if updated.Status.DhcpStatus == nil {
		updated.Status.DhcpStatus = &topohubv1beta1.DhcpStatusSpec{}
	}
	updated.Status.DhcpStatus.IpTotalAmount = int32(s.totalIPs)
	updated.Status.DhcpStatus.IpAssignAmount = int32(len(currentClients))
	updated.Status.DhcpStatus.IpAvailableAmount = int32(s.totalIPs - len(currentClients))

	// 发送状态更新
	s.statusUpdateCh <- updated

	// 更新 dnsmasq 配置
	if s.subnet.Spec.Feature.EnableBindDhcpIP {
		configFilePath, err := s.generateDnsmasqConfig(currentClients)
		if err != nil {
			s.log.Errorf("Failed to update dnsmasq config: %v", err)
			return err
		}

		// 重新加载 dnsmasq 配置
		if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
			return fmt.Errorf("failed to reload dnsmasq: %v", err)
		}
		s.log.Infof("Reloaded dnsmasq config: %s", configFilePath)
	}

	return nil
}

// watchLeaseStatus monitors the lease file and updates status
func (s *dhcpServer) watchLeaseStatus() {
	leaseFile := filepath.Join(s.config.StoragePathDhcpLease, fmt.Sprintf("dnsmasq-%s.leases", s.subnet.Name))

	// 创建文件监控器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Errorf("Failed to create lease file watcher: %v", err)
		return
	}
	defer watcher.Close()

	// 添加监控文件
	if err := watcher.Add(filepath.Dir(leaseFile)); err != nil {
		s.log.Errorf("Failed to watch lease file: %v", err)
		return
	}

	// 开始监控
	for {
		select {
		case <-s.stopCh:
			return
		case event := <-watcher.Events:
			if event.Name == leaseFile && (event.Op&fsnotify.Write == fsnotify.Write) {
				if err := s.processLeaseFile(leaseFile); err != nil {
					s.log.Errorf("Failed to process lease file: %v", err)
				}
			}
		case err := <-watcher.Errors:
			s.log.Errorf("Lease file watcher error: %v", err)
		}
	}
}
