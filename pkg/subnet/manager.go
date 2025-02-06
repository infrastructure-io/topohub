package subnet

import (
	"context"
	"time"

	"github.com/infrastructure-io/topohub/pkg/lock"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/subnet/dhcpserver"
)

type SubnetManager interface {
	SetupWithManager(mgr ctrl.Manager) error
	Stop()
	GetDhcpClientEvents() (chan dhcpserver.DhcpClientInfo, chan dhcpserver.DhcpClientInfo)
}

type subnetManager struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
	cache      *SubnetCache

	addedDhcpClient   chan dhcpserver.DhcpClientInfo
	deletedDhcpClient chan dhcpserver.DhcpClientInfo

	// lock
	lockLeader     lock.RWMutex
	leader         bool
	dhcpServerList map[string]dhcpserver.DhcpServer
}

func NewSubnetReconciler(config config.AgentConfig, kubeClient kubernetes.Interface) SubnetManager {
	return &subnetManager{
		config:            &config,
		kubeClient:        kubeClient,
		cache:             NewSubnetCache(),
		lockLeader:        lock.RWMutex{},
		addedDhcpClient:   make(chan dhcpserver.DhcpClientInfo, 100),
		deletedDhcpClient: make(chan dhcpserver.DhcpClientInfo, 100),
		leader:            false,
		dhcpServerList:    make(map[string]dhcpserver.DhcpServer),
	}
}

// Reconcile handles the reconciliation of Subnet objects
func (s *subnetManager) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.Logger.With(
		zap.String("subnet", req.Name),
	)

	// Get the Subnet instance
	subnet := &topohubv1beta1.Subnet{}
	if err := s.client.Get(ctx, req.NamespacedName, subnet); err != nil {
		if k8serrors.IsNotFound(err) {
			// Subnet was deleted
			logger.Infof("Subnet %s was deleted, removing from cache", req.Name)
			s.cache.Delete(req.Name)
			if server, exists := s.dhcpServerList[req.Name]; exists {
				logger.Infof("Stopping DHCP server for subnet %s", req.Name)
				server.Stop()
				delete(s.dhcpServerList, req.Name)
			}
			return reconcile.Result{}, nil
		}
		logger.Errorf("Failed to get Subnet %s: %v", req.Name, err)
		return reconcile.Result{}, err
	}

	// Check if this is a new subnet or if the spec has changed
	s.lockLeader.Lock()
	defer s.lockLeader.Unlock()
	// if we are the leader, we should handle the subnet
	if s.leader {
		if s.cache.HasSpecChanged(subnet) {
			logger.Infof("Subnet %s spec changed or new subnet detected (subnet: %s, ipRange: %s)",
				subnet.Name,
				subnet.Spec.IPv4Subnet.Subnet,
				subnet.Spec.IPv4Subnet.IPRange)

			// todo: start the dhcp server on the subnet
			if _, ok := s.dhcpServerList[subnet.Name]; !ok {
				t := dhcpserver.NewDhcpServer(s.config, subnet, s.client, s.addedDhcpClient, s.deletedDhcpClient)
				err := t.Run()
				if err != nil {
					logger.Errorf("Failed to start DHCP server for subnet %s: %v", subnet.Name, err)
					return reconcile.Result{
						RequeueAfter: time.Second * 2,
					}, err
				} else {
					logger.Infof("Started DHCP server for subnet %s", subnet.Name)
					// Update the cache with the latest version
					s.dhcpServerList[subnet.Name] = t
					s.cache.Set(subnet)
				}
			} else {
				logger.Infof("updated DHCP server for subnet %s", subnet.Name)
				s.dhcpServerList[subnet.Name].UpdateService(*subnet)
				s.cache.Set(subnet)
			}
		} else {
			logger.Debugf("Subnet %s spec has no change", subnet.Name)
		}
	}

	return reconcile.Result{}, nil
}

func (s *subnetManager) SetupWithManager(mgr ctrl.Manager) error {
	s.client = mgr.GetClient()

	// start all dhcp server when we are the leader
	go func() {
		log.Logger.Info("waiting for the election of leader")
		select {
		case <-mgr.Elected():
			log.Logger.Info("Elected as leader, begin to start all controllers")

			s.lockLeader.Lock()
			defer s.lockLeader.Unlock()
			s.leader = true

			// 获取所有的 Subnet 实例并启动 DHCP 服务器
			var subnetList topohubv1beta1.SubnetList
			if err := mgr.GetClient().List(context.Background(), &subnetList); err != nil {
				log.Logger.Errorf("Failed to list subnets: %v", err)
				return
			}

			// 初始化并启动 DHCP 服务器
			for _, subnet := range subnetList.Items {
				if subnet.DeletionTimestamp != nil {
					continue
				}

				// 检查是否已经存在对应的 DHCP 服务器
				if _, exists := s.dhcpServerList[subnet.Name]; !exists {
					// 创建新的 DHCP 服务器实例
					dhcpServer := dhcpserver.NewDhcpServer(s.config, &subnet, s.client, s.addedDhcpClient, s.deletedDhcpClient)

					// 启动 DHCP 服务器
					if err := dhcpServer.Run(); err != nil {
						log.Logger.Errorf("Failed to start DHCP server for subnet %s: %v", subnet.Name, err)
					} else {
						log.Logger.Infof("Started DHCP server for subnet %s", subnet.Name)
						s.dhcpServerList[subnet.Name] = dhcpServer
					}
				}
			}
		}
	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.Subnet{}).
		Complete(s)
}

func (s *subnetManager) Stop() {
	// Clean up any resources if needed
	for name, server := range s.dhcpServerList {
		log.Logger.Infof("Stopping DHCP server for subnet %s", name)
		server.Stop()
	}
}

func (s *subnetManager) GetDhcpClientEvents() (chan dhcpserver.DhcpClientInfo, chan dhcpserver.DhcpClientInfo) {
	return s.addedDhcpClient, s.deletedDhcpClient
}
