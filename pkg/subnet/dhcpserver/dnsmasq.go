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

	// create context for process management
	ctx, cancel := context.WithCancel(context.Background())
	s.cmdCancel = cancel

	// start dnsmasq
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

	// wait for process to start
	time.Sleep(time.Second)

	// check if the process is running
	if cmd.Process == nil {
		return fmt.Errorf("dnsmasq process failed to start")
	}

	// update the status of subnet
	s.statusUpdateCh <- struct{}{}

	return nil
}

// UpdateBindingIpEvents方法已移回manager.go

// isDnsmasqProcessDead checks if the dnsmasq process is dead
func (s *dhcpServer) isDnsmasqProcessDead() bool {
	// check if the process exists
	isDead := s.cmd == nil || s.cmd.Process == nil

	// if the process exists, try to send signal 0 to check the process status
	if !isDead {
		if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			s.log.Debugf("Process exists but signal check failed: %v", err)
			return true
		}
	}

	return isDead
}

// stopDnsmasqProcess stops the dnsmasq process
func (s *dhcpServer) stopDnsmasqProcess() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.log.Infof("Stopping existing dnsmasq process")
		if err := s.cmd.Process.Kill(); err != nil {
			s.log.Warnf("Failed to kill dnsmasq process: %v", err)
		}

		// cancel the context
		if s.cmdCancel != nil {
			s.cmdCancel()
		}
	}
}

// monitor monitors the lease file and updates status
func (s *dhcpServer) monitor() {
	leaseWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Errorf("failed to create lease file watcher: %v", err)
		return
	}
	defer func() {
		if err := leaseWatcher.Close(); err != nil {
			s.log.Errorf("failed to close lease file watcher: %v", err)
		}
	}()

	if err := leaseWatcher.Add(filepath.Dir(s.leasePath)); err != nil {
		s.log.Errorf("failed to watch lease file: %v", err)
		return
	}

	// watch the process at an interval
	tickerProcess := time.NewTicker(5 * time.Second)
	defer tickerProcess.Stop()

	s.lockData.RLock()
	subnetName := s.subnet.Name
	s.lockData.RUnlock()

	// start to monitor
	for {
		needRestart := false
		needReload := false
		needRenewConfig := false

		select {
		case <-s.stopCh:
			s.log.Errorf("subnet monitor is exiting")
			return

		// watch error of lease file
		case err := <-leaseWatcher.Errors:
			s.log.Errorf("Lease file watcher error: %v", err)

		// lease file event
		case event, ok := <-leaseWatcher.Events:
			if !ok {
				s.log.Panicf("Lease file watcher channel closed")
			}

			if event.Name == s.leasePath && (event.Op&fsnotify.Write == fsnotify.Write) {
				s.log.Infof("watcher lease file event: %+v", event)
				// inform new client to the redfishStatu
				if _, err := s.processDhcpLease(true); err != nil {
					s.log.Errorf("failed to processDhcpLease: %v", err)
					continue
				}
				// update the status of subnet
				s.statusUpdateCh <- struct{}{}
			} else {
				s.log.Debugf("watcher invalid file event: %+v", event)
			}

		case info := <-s.addedBindingIp:
			s.log.Debugf("process binding ip adding events for subnet %s: %+v", info.Subnet, info)
			// note: currently, it does not consider whether the ip is belonged to the ip range or not, which make it simple to handle the subnet changes
			if item, ok := s.currentManualBindingClients[info.IPAddr]; ok {
				if item.MAC != info.MacAddr {
					s.lockData.Lock()
					s.currentManualBindingClients[info.IPAddr] = &DhcpClientInfo{
						IP:       info.IPAddr,
						MAC:      info.MacAddr,
						Hostname: info.Hostname,
					}
					s.lockData.Unlock()
					s.log.Infof("update binding ip %s: old mac %s, new mac %s, hostname %s", info.IPAddr, item.MAC, info.MacAddr, info.Hostname)
				} else {
					continue
				}
			} else {
				s.lockData.Lock()
				s.currentManualBindingClients[info.IPAddr] = &DhcpClientInfo{
					IP:       info.IPAddr,
					MAC:      info.MacAddr,
					Hostname: info.Hostname,
				}
				s.lockData.Unlock()
				s.log.Infof("add new binding ip %s: %+v", info.IPAddr, info)
			}
			if err := s.UpdateDhcpBindings(); err != nil {
				s.log.Errorf("failed to add dhcp bindings: %v", err)
				continue
			}
			// it has been renew the config
			needRenewConfig = false
			needReload = true

		case info := <-s.deletedBindingIp:
			s.log.Debugf("process binding ip deleting events for subnet %s: %+v", info.Subnet, info)
			// note: currently, it does not consider whether the ip is belonged to the ip range or not, which make it simple to handle the subnet changes
			if item, ok := s.currentManualBindingClients[info.IPAddr]; ok && strings.EqualFold(item.MAC, info.MacAddr) {
				delete(s.currentManualBindingClients, info.IPAddr)
				s.log.Infof("delete binding ip %s: %+v", info.IPAddr, info)
			} else {
				continue
			}
			if err := s.UpdateDhcpBindings(); err != nil {
				s.log.Errorf("failed to delete dhcp bindings: %v", err)
				continue
			}
			// it has been renew the config
			needRenewConfig = false
			needReload = true

		// reconcile notify subnet spec changes
		case force := <-s.restartCh:
			needRenewConfig = true
			if force {
				s.log.Infof("dhcp server restart after the spec of subnet is updated")
				needRestart = true
			} else {
				s.log.Infof("dhcp server reload after the spec of subnet is updated")
				needReload = true
			}

		// check the process
		case <-tickerProcess.C:
			// check if the process is running
			if s.isDnsmasqProcessDead() {
				s.log.Infof("DHCP server process not running, starting")
				needRestart = true
				needRenewConfig = true
			} else {
				s.log.Debugf("dhcp server for %s is running", subnetName)
			}
		}

		// renew the config
		if needRenewConfig {
			if err := s.generateDnsmasqConfig(); err != nil {
				s.log.Errorf("Failed to update dnsmasq config: %v", err)
				continue
			}
		}

		// check the process
		isDead := s.isDnsmasqProcessDead()

		// if the process is dead, restart it
		if isDead && needReload {
			s.log.Infof("DHCP server process not running, restarting instead of reloading")
			needRestart = true
			needReload = false
		}

		// handle restart logic
		if needRestart {
			s.log.Infof("restarting dhcp server")

			// stop the existing process (if exists)
			if !isDead {
				s.stopDnsmasqProcess()
			}

			// start the new process
			if err := s.startDnsmasq(); err != nil {
				s.log.Errorf("Failed to restart dnsmasq: %v", err)
			}

			// handle restart logic
		} else if needReload {
			s.log.Infof("reload dhcp server")
			// reload the config
			if err := s.cmd.Process.Signal(syscall.SIGHUP); err != nil {
				s.log.Errorf("failed to reload dnsmasq: %v, will try to restart the process", err)
				// stop the existing process (if exists)
				s.stopDnsmasqProcess()
				// start the new process
				if err := s.startDnsmasq(); err != nil {
					s.log.Errorf("Failed to restart dnsmasq after reload failure: %v", err)
				}
			} else {
				s.log.Infof("Reloaded dnsmasq config: %s", s.configPath)
				// update the status of subnet
				s.statusUpdateCh <- struct{}{}
			}
		}
	}
}
