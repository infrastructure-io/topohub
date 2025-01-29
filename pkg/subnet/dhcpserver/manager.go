package dhcpserver

import (
	"context"
	"fmt"
	"os/exec"

	"go.uber.org/zap"

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
}

type dhcpServer struct {
	config            *config.AgentConfig
	subnet            *topohubv1beta1.Subnet
	client            client.Client
	cmd               *exec.Cmd
	cmdCancel         context.CancelFunc
	stopCh            chan struct{}
	addedDhcpClient   chan DhcpClientInfo
	deletedDhcpClient chan DhcpClientInfo
	stats             *IPUsageStats
	mu                *lock.RWMutex
	statusUpdateCh    chan *topohubv1beta1.Subnet
	log               *zap.SugaredLogger
	currentClients    map[string]*DhcpClientInfo
	totalIPs          int
}

// NewDhcpServer creates a new DHCP server instance
func NewDhcpServer(config *config.AgentConfig, subnet *topohubv1beta1.Subnet, client client.Client, addedDhcpClient chan DhcpClientInfo, deletedDhcpClient chan DhcpClientInfo) (*dhcpServer, error) {
	server := &dhcpServer{
		config:            config,
		subnet:            subnet,
		client:            client,
		addedDhcpClient:   addedDhcpClient,
		deletedDhcpClient: deletedDhcpClient,
		stopCh:            make(chan struct{}),
		stats:             &IPUsageStats{},
		mu:                &lock.RWMutex{},
		statusUpdateCh:    make(chan *topohubv1beta1.Subnet, 100),
		log:               log.Logger.With(zap.String("subnet", subnet.Name)),
		currentClients:    make(map[string]*DhcpClientInfo),
		totalIPs:          0,
	}

	return server, nil
}

// Run starts the DHCP server and all associated services
func (s *dhcpServer) Run() error {
	// 清理可能存在的旧接口
	if err := s.cleanupAllInterface(); err != nil {
		s.log.Warnf("Failed to cleanup old interface: %v", err)
	}

	// 启动状态更新协程
	go s.statusUpdateWorker()

	// 启动 DHCP 服务
	if err := s.restartDhcpServer(); err != nil {
		return fmt.Errorf("failed to start DHCP server: %v", err)
	}

	// 启动状态监控
	go s.watchLeaseStatus()

	return nil
}

// restartDhcpServer configures and starts the dnsmasq service
func (s *dhcpServer) restartDhcpServer() error {
	// 1. 配置网络接口
	if err := s.setupInterface(); err != nil {
		return fmt.Errorf("failed to setup interface: %v", err)
	}
	//  启动 dnsmasq
	return s.startDnsmasq()
}

// Stop stops all services and cleans up resources
func (s *dhcpServer) Stop() error {
	close(s.stopCh)

	// 停止 dnsmasq 进程
	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Kill(); err != nil {
			s.log.Errorf("Failed to kill dnsmasq process: %v", err)
		}
	}

	// 清理网络接口
	if err := s.cleanupAllInterface(); err != nil {
		s.log.Errorf("Failed to cleanup network interface: %v", err)
	}

	return nil
}
