package dhcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// startDnsmasq starts the dnsmasq process
func (s *dhcpServer) startDnsmasq() error {

	if err := s.setupInterface(); err != nil {
		return fmt.Errorf("failed to setup interface: %v", err)
	}

	err := s.generateDnsmasqConfig()
	if err != nil {
		return fmt.Errorf("failed to generate dnsmasq config: %v", err)
	}
	s.log.Infof("dnsmasq config file %s", s.configPath)

	// 创建 context 用于进程管理
	ctx, cancel := context.WithCancel(context.Background())
	s.cmdCancel = cancel

	// 启动 dnsmasq
	cmd := exec.Command("dnsmasq", "-C", s.configPath, "-d")
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

	// update the status of subnet
	s.statusUpdateCh <- struct{}{}

	return nil
}

// UpdateService updates the subnet configuration and restarts the DHCP server
func (s *dhcpServer) UpdateService(subnet topohubv1beta1.Subnet) error {
	s.lockData.Lock()
	defer s.lockData.Unlock()

	// 更新 subnet
	s.subnet = &subnet
	// 重启 DHCP 服务
	s.restartCh <- struct{}{}

	return nil
}

// monitor monitors the lease file and updates status
func (s *dhcpServer) monitor() {

	watcher := &fsnotify.Watcher{Events: make(chan fsnotify.Event, 0)}
	var err error
	if s.subnet.Spec.Feature.EnableBindDhcpIP {
		s.log.Infof(" bind dhcp ip is enabled, and watch lease file")
		// 添加 lease 文件监控
		watcher, err = fsnotify.NewWatcher()
		if err != nil {
			s.log.Errorf("Failed to create lease file watcher: %v", err)
			return
		}
		defer watcher.Close()
		if err := watcher.Add(filepath.Dir(s.leasePath)); err != nil {
			s.log.Errorf("Failed to watch lease file: %v", err)
			return
		}
	} else {
		s.log.Infof("bind dhcp ip is disabled, and do not watch lease file")
	}

	// watch the process at an interval
	tickerProcess := time.NewTicker(3 * time.Second)
	defer tickerProcess.Stop()

	// 开始监控
	for {
		needRestart := false
		needReload := false
		needRenewConfig := false

		select {
		case <-s.stopCh:
			s.log.Errorf("subnet monitor is exiting")
			return

		// watch error of lease file
		case err := <-watcher.Errors:
			s.log.Errorf("Lease file watcher error: %v", err)

		// lease file event
		case event, ok := <-watcher.Events:
			if !ok {
				s.log.Panicf("Lease file watcher channel closed")
			}
			s.log.Debugf("watcher event: %+v", event)
			if event.Name == s.leasePath && (event.Op&fsnotify.Write == fsnotify.Write) {
				if reloadConfig, err := s.processLeaseAndUpdateBindings(true); err != nil {
					s.log.Errorf("failed to processLeaseAndUpdateBindings: %v", err)
				} else {
					if reloadConfig {
						needReload = true
						needRenewConfig = true
						s.log.Infof("client ip or mac changed, so dhcp server reload after binding new ip")
					} else {
						s.log.Infof("client expiration is updated, so dhcp server does not need to reload")
					}
				}
			}

		case event, ok := <-s.deletedHostStatus:
			if !ok {
				s.log.Panic("deletedHostStatus channel closed")
			}
			s.log.Debugf("process hostStatus deleting events, delete dhcp binding, ip %s, mac %s", event.IP, event.MAC)
			if err := s.UpdateDhcpBindings(nil, map[string]string{event.IP: event.MAC}); err != nil {
				s.log.Errorf("failed to delete dhcp binding for ip %s, err: %v", event.IP, err)
				continue
			}
			needReload = true
			// it has been renew the config
			needRenewConfig = false

		// reconcile notify subnet spec changes
		case <-s.restartCh:
			needReload = true
			needRenewConfig = true
			s.log.Infof("dhcp server reload after the spec of subnet is updated")

		// check the process
		case <-tickerProcess.C:
			isDead := s.cmd == nil || s.cmd.Process == nil
			if !isDead {
				if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
					s.log.Errorf("DHCP server process check failed: %v", err)
					needRestart = true
					needRenewConfig = true
				} else {
					s.log.Debugf("dhcp server for %s is running", s.subnet.Name)
				}
			} else {
				needRestart = true
				needRenewConfig = true
				s.log.Infof("dhcp server for %s is dead, restart it", s.subnet.Name)
			}
		}

		if needRenewConfig {
			if err := s.generateDnsmasqConfig(); err != nil {
				s.log.Errorf("Failed to update dnsmasq config: %v", err)
				continue
			}
		}

		if needReload {
			s.log.Infof("reload dhcp server")
			// 重新加载 dnsmasq 配置
			if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
				s.log.Errorf("failed to reload dnsmasq: %v", err)
				continue
			}
			s.log.Infof("Reloaded dnsmasq config: %s", s.configPath)
			// update the status of subnet
			s.statusUpdateCh <- struct{}{}

		} else if needRestart {
			s.log.Infof("restarting dhcp server")
			s.startDnsmasq()
			// update the status of subnet
			s.statusUpdateCh <- struct{}{}
		}

	}
}
