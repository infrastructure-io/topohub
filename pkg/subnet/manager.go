package subnet

import (
	"context"
	"fmt"
	"time"

	bindingipdata "github.com/infrastructure-io/topohub/pkg/bindingip/data"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/infrastructure-io/topohub/pkg/lock"
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
	GetDhcpClientEventsForHostStatus() (chan dhcpserver.DhcpClientInfo, chan dhcpserver.DhcpClientInfo)
	GetHostStatusEvents() chan dhcpserver.DhcpClientInfo
	GetBindingIpEvents() (chan bindingipdata.BindingIPInfo, chan bindingipdata.BindingIPInfo)
}

type subnetManager struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
	cache      *SubnetCache

	log *zap.SugaredLogger

	// 本模块往其中添加数据，关于 dhcp client 变化信息。由 hoststatus 模块来消费使用
	addedDhcpClientForHostStatus   chan dhcpserver.DhcpClientInfo
	deletedDhcpClientForHostStatus chan dhcpserver.DhcpClientInfo

	// hoststatus 往其中添加数据，关于 hoststatus 被删除信息。由本模块来消费使用
	deletedHostStatus chan dhcpserver.DhcpClientInfo

	// bindingip 模块 往其中添加数据，关于 bindingip 。由本模块来消费使用
	addedBindingIp   chan bindingipdata.BindingIPInfo
	deletedBindingIp chan bindingipdata.BindingIPInfo

	// lock
	lockLeader     lock.RWMutex
	leader         bool
	dhcpServerList map[string]dhcpserver.DhcpServer
}

func NewSubnetReconciler(config config.AgentConfig, kubeClient kubernetes.Interface) SubnetManager {
	return &subnetManager{
		config:                         &config,
		kubeClient:                     kubeClient,
		cache:                          NewSubnetCache(),
		lockLeader:                     lock.RWMutex{},
		addedDhcpClientForHostStatus:   make(chan dhcpserver.DhcpClientInfo, 1000),
		deletedDhcpClientForHostStatus: make(chan dhcpserver.DhcpClientInfo, 1000),
		deletedHostStatus:              make(chan dhcpserver.DhcpClientInfo, 1000),
		addedBindingIp:                 make(chan bindingipdata.BindingIPInfo, 1000),
		deletedBindingIp:               make(chan bindingipdata.BindingIPInfo, 1000),
		leader:                         false,
		dhcpServerList:                 make(map[string]dhcpserver.DhcpServer),
		log:                            log.Logger.Named("subnetManager"),
	}
}

// update the status
func (s *subnetManager) UpdateSubnetStatus(subnet *topohubv1beta1.Subnet, reason, errorMsg string, logger *zap.SugaredLogger) (reconcile.Result, error) {

	updated := subnet.DeepCopy()
	if updated.Status.Conditions == nil {
		updated.Status.Conditions = []metav1.Condition{}
	}
	updated.Status.Conditions = append(updated.Status.Conditions, metav1.Condition{
		Type:               "DhcpServer",
		Reason:             reason,
		Message:            errorMsg,
		Status:             "False",
		LastTransitionTime: metav1.Now(),
	})

	if err := s.client.Status().Update(context.TODO(), updated); err != nil {
		logger.Errorf("failed to update status: %v", err)
		return reconcile.Result{
			RequeueAfter: time.Second * 2,
		}, err
	}
	s.log.Infof("succeeded to update subnet status for %s: %v", updated.ObjectMeta.Name, updated.Status.DhcpStatus)

	return reconcile.Result{}, nil
}

// Reconcile handles the reconciliation of Subnet objects
func (s *subnetManager) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := s.log.With(zap.String("subnet", req.Name))

	// Get the Subnet instance
	subnet := &topohubv1beta1.Subnet{}
	if err := s.client.Get(ctx, req.NamespacedName, subnet); err != nil {
		if k8serrors.IsNotFound(err) {
			// Subnet was deleted
			logger.Infof("Subnet %s was deleted, removing from cache", req.Name)
			s.cache.Delete(req.Name)
			if server, exists := s.dhcpServerList[req.Name]; exists {
				logger.Infof("Stopping DHCP server for subnet %s", req.Name)
				if err := server.Stop(); err != nil {
					logger.Errorf("Failed to stop DHCP server for subnet %s: %v", req.Name, err)
				}
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
				t := dhcpserver.NewDhcpServer(s.config, subnet, s.client, s.addedDhcpClientForHostStatus, s.deletedDhcpClientForHostStatus)
				err := t.Run()
				if err != nil {
					msg := fmt.Sprintf("Failed to start DHCP server for subnet %s: %v", subnet.Name, err)
					logger.Errorf(msg)
					return s.UpdateSubnetStatus(subnet, "Failed", msg, logger)
				} else {
					logger.Infof("Started DHCP server for subnet %s", subnet.Name)
					// Update the cache with the latest version
					s.dhcpServerList[subnet.Name] = t
					s.cache.Set(subnet)
				}

				// get all binding ip
				bindingIPInfoList := bindingipdata.BindingIPCacheDatabase.GetInfoForSubnet(subnet.Name)
				if len(bindingIPInfoList) > 0 {
					logger.Infof("add binding ip events for subnet %s: %+v", subnet.Name, bindingIPInfoList)
					if err := s.dhcpServerList[subnet.Name].UpdateBindingIpEvents(bindingIPInfoList, nil); err != nil {
						msg := fmt.Sprintf("Failed to update binding ip events for subnet %s: %v", subnet.Name, err)
						logger.Errorf(msg)
						return s.UpdateSubnetStatus(subnet, "Failed", msg, logger)
					}
				}

			} else {
				logger.Infof("updated DHCP server for subnet %s", subnet.Name)
				if err := s.dhcpServerList[subnet.Name].UpdateService(*subnet); err != nil {
					msg := fmt.Sprintf("Failed to update DHCP service for subnet %s: %v", subnet.Name, err)
					logger.Errorf(msg)
					return s.UpdateSubnetStatus(subnet, "Failed", msg, logger)
				}
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
		<-mgr.Elected()
		s.log.Info("Elected as leader, begin to start all controllers")

		s.lockLeader.Lock()
		defer s.lockLeader.Unlock()
		s.leader = true

		// 获取所有的 Subnet 实例并启动 DHCP 服务器
		var subnetList topohubv1beta1.SubnetList
		if err := mgr.GetClient().List(context.Background(), &subnetList); err != nil {
			s.log.Errorf("Failed to list subnets: %v", err)
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
				dhcpServer := dhcpserver.NewDhcpServer(s.config, &subnet, s.client, s.addedDhcpClientForHostStatus, s.deletedDhcpClientForHostStatus)

				// 启动 DHCP 服务器
				if err := dhcpServer.Run(); err != nil {
					s.log.Errorf("Failed to start DHCP server for subnet %s: %v", subnet.Name, err)
				} else {
					s.log.Infof("Started DHCP server for subnet %s", subnet.Name)
					s.dhcpServerList[subnet.Name] = dhcpServer
				}
			}
		}

		// after all server is started , start to process binding ip event
		time.Sleep(2 * time.Second)
		go s.processBindingIpEvents()
		// Start subnet manager
		go s.processHostStatusEvents()

	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.Subnet{}).
		Complete(s)
}

func (s *subnetManager) Stop() {
	s.log.Info("Stopping subnet manager")
	for name, server := range s.dhcpServerList {
		if err := server.Stop(); err != nil {
			s.log.Errorf("Failed to stop DHCP server for subnet %s: %v", name, err)
		}
	}
}

// this module send event to the channel, and hoststatus module consume it
func (s *subnetManager) GetDhcpClientEventsForHostStatus() (chan dhcpserver.DhcpClientInfo, chan dhcpserver.DhcpClientInfo) {
	return s.addedDhcpClientForHostStatus, s.deletedDhcpClientForHostStatus
}

// hoststatus module send event to this channel and this module consume it
func (s *subnetManager) GetHostStatusEvents() chan dhcpserver.DhcpClientInfo {
	return s.deletedDhcpClientForHostStatus
}

// DHCP manager 把 dhcp client 事件告知后，进行 hoststatus 更新
func (s *subnetManager) processHostStatusEvents() {
	s.log.Infof("begin to process host status events for deleting binding setting")

	for event := range s.deletedDhcpClientForHostStatus {
		s.log.Debugf("process host status deleted events: %+v", event)
		if c, exists := s.dhcpServerList[event.SubnetName]; !exists {
			s.log.Errorf("subnet %s is not running, skip to process host status events: %+v", event.SubnetName, event)
		} else {
			if err := c.DeleteDhcpBinding(event.IP, event.MAC); err != nil {
				s.log.Errorf("failed to delete dhcp binding: %v", err)
			}
		}
	}
	s.log.Panic("deletedDhcpClient channel closed")
}

func (s *subnetManager) GetBindingIpEvents() (chan bindingipdata.BindingIPInfo, chan bindingipdata.BindingIPInfo) {
	return s.addedBindingIp, s.deletedBindingIp
}

func (s *subnetManager) processBindingIpEvents() {

	// handle bindingIP crd events, and configure it in the dhcp server
	s.log.Infof("begin to process binding ip events")
	for {
		select {
		case event := <-s.addedBindingIp:
			s.log.Debugf("receive adding binding ip event: %+v", event)
			if c, exists := s.dhcpServerList[event.Subnet]; !exists {
				s.log.Errorf("subnet %s is not running, skip to process binding ip events: %+v", event.Subnet, event)
				go func() {
					time.Sleep(30 * time.Second)
					s.addedBindingIp <- event
				}()
			} else {
				s.log.Infof("process binding ip adding events for subnet %s: %+v", event.Subnet, event)
				if err := c.UpdateBindingIpEvents([]bindingipdata.BindingIPInfo{event}, nil); err != nil {
					s.log.Errorf("failed to add dhcp binding: %v", err)
				}
			}

		case event := <-s.deletedBindingIp:
			s.log.Debugf("receive deleting binding ip event: %+v", event)
			if c, exists := s.dhcpServerList[event.Subnet]; !exists {
				s.log.Errorf("subnet %s is not running, skip to process binding ip events: %+v", event.Subnet, event)
				go func() {
					time.Sleep(30 * time.Second)
					s.deletedBindingIp <- event
				}()
			} else {
				s.log.Infof("process binding ip deleting events for subnet %s: %+v", event.Subnet, event)
				if err := c.UpdateBindingIpEvents(nil, []bindingipdata.BindingIPInfo{event}); err != nil {
					s.log.Errorf("failed to delete dhcp binding: %v", err)
				}
			}
		}
	}
}
