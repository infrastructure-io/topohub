package dhcpserver

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"go.uber.org/zap"

	bindingipdata "github.com/infrastructure-io/topohub/pkg/bindingip/data"
	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DhcpServer defines the interface for DHCP server operations
type DhcpServer interface {
	// Run starts the DHCP server
	Run() error

	// Stop stops the DHCP server
	Stop() error

	// UpdateService updates the subnet configuration
	UpdateService(subnet topohubv1beta1.Subnet) error

	// DeleteDhcpBinding deletes the DHCP binding for the specified IP and MAC
	DeleteDhcpBinding(ip, mac string) error

	// UpdateBindingIpEvents updates the binding IP events
	UpdateBindingIpEvents(added []bindingipdata.BindingIPInfo, deleted []bindingipdata.BindingIPInfo) error
}

type dhcpServer struct {
	config *config.AgentConfig
	client client.Client

	lockData *lock.RWMutex
	subnet   *topohubv1beta1.Subnet

	currentLeaseClients         map[string]*DhcpClientInfo
	currentManualBindingClients map[string]*DhcpClientInfo
	currentAutoBindingClients   map[string]*DhcpClientInfo

	lockConfigUpdate *lock.RWMutex

	//
	cmd       *exec.Cmd
	cmdCancel context.CancelFunc
	stopCh    chan struct{}
	// 本模块用来通知给 HostStatus 模块，有新的 dhcp ip 分配，让其创建出 hoststatus
	addedDhcpClientForHostStatus chan DhcpClientInfo
	// 本模块用来通知给 HostStatus 模块，有 dhcp ip 释放
	deletedDhcpClientForHostStatus chan DhcpClientInfo
	// 	HostStatus 模块通知来的 HostStatus 删除事件，进行 ip 解绑处理
	deletedHostStatus chan DhcpClientInfo

	// bindingip 模块 往其中添加数据，关于 bindingip 。由本模块来消费使用
	addedBindingIp   chan bindingipdata.BindingIPInfo
	deletedBindingIp chan bindingipdata.BindingIPInfo

	// update the status of crd
	statusUpdateCh chan struct{}
	log            *zap.SugaredLogger

	// restart the dhcp server
	restartCh chan struct{}

	// file path
	configTemplatePath       string
	configPath               string
	HostIpBindingsConfigPath string
	leasePath                string
	logPath                  string
}

// NewDhcpServer creates a new DHCP server instance
func NewDhcpServer(config *config.AgentConfig, subnet *topohubv1beta1.Subnet, client client.Client, addedDhcpClientForHostStatus chan DhcpClientInfo, deletedDhcpClientForHostStatus chan DhcpClientInfo) *dhcpServer {

	return &dhcpServer{
		config:                         config,
		lockData:                       &lock.RWMutex{},
		lockConfigUpdate:               &lock.RWMutex{},
		subnet:                         subnet,
		client:                         client,
		addedDhcpClientForHostStatus:   addedDhcpClientForHostStatus,
		deletedDhcpClientForHostStatus: deletedDhcpClientForHostStatus,
		deletedHostStatus:              make(chan DhcpClientInfo, 1000),
		addedBindingIp:                 make(chan bindingipdata.BindingIPInfo, 1000),
		deletedBindingIp:               make(chan bindingipdata.BindingIPInfo, 1000),
		stopCh:                         make(chan struct{}),
		statusUpdateCh:                 make(chan struct{}),
		restartCh:                      make(chan struct{}),
		log:                            log.Logger.Named("dhcpServer/" + subnet.Name),
		currentLeaseClients:            make(map[string]*DhcpClientInfo),
		currentManualBindingClients:    make(map[string]*DhcpClientInfo),
		currentAutoBindingClients:      make(map[string]*DhcpClientInfo),
		configTemplatePath:             filepath.Join(config.DhcpConfigTemplatePath, "dnsmasq.conf.tmpl"),
		configPath:                     filepath.Join(config.StoragePathDhcpConfig, fmt.Sprintf("dnsmasq-%s.conf", subnet.Name)),
		HostIpBindingsConfigPath:       filepath.Join(config.StoragePathDhcpConfig, fmt.Sprintf("dnsmasq-%s-bindIp.conf", subnet.Name)),
		leasePath:                      filepath.Join(config.StoragePathDhcpLease, fmt.Sprintf("dnsmasq-%s.leases", subnet.Name)),
		logPath:                        filepath.Join(config.StoragePathDhcpLog, fmt.Sprintf("dnsmasq-%s.log", subnet.Name)),
	}
}

// Run starts the DHCP server and all associated services
func (s *dhcpServer) Run() error {
	s.log.Infof("run whole dhcp server service")

	// 清理可能存在的旧接口
	if err := s.cleanupAllInterface(); err != nil {
		s.log.Warnf("Failed to cleanup old interface: %v", err)
	}

	// 启动 CRD 更新协程
	go s.statusUpdateWorker()

	// 启动 DHCP 服务
	if err := s.startDnsmasq(); err != nil {
		return fmt.Errorf("failed to start DHCP server: %v", err)
	}

	// 启动状态监控
	go s.monitor()

	s.log.Infof("finished setting up dhcp server")

	return nil
}

// Stop stops all services and cleans up resources
func (s *dhcpServer) Stop() error {
	s.log.Infof("stop whole dhcp server service")

	// 停止 dnsmasq 进程
	close(s.stopCh)
	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Kill(); err != nil {
			s.log.Errorf("Failed to kill dnsmasq process: %v", err)
		}
	}

	// 清理网络接口
	s.log.Infof("clean all interfaces")
	if err := s.cleanupAllInterface(); err != nil {
		s.log.Errorf("Failed to cleanup network interface: %v", err)
	}

	return nil
}

func (s *dhcpServer) DeleteDhcpBinding(ip, mac string) error {
	s.deletedHostStatus <- DhcpClientInfo{IP: ip, MAC: mac}
	return nil
}

func (s *dhcpServer) UpdateBindingIpEvents(added []bindingipdata.BindingIPInfo, deleted []bindingipdata.BindingIPInfo) error {
	for _, info := range added {
		s.addedBindingIp <- info
	}
	for _, info := range deleted {
		s.deletedBindingIp <- info
	}
	return nil
}
