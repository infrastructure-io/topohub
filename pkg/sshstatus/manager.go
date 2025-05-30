package sshstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// SSHStatusController 接口定义了SSH状态控制器的公共方法
type SSHStatusController interface {
	Stop()
	SetupWithManager(ctrl.Manager) error
	// 更新SSH主机的认证信息
	UpdateSecret(string, string, string, string)
}

// sshStatusController 实现了SSHStatusController接口
type sshStatusController struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
	stopCh     chan struct{}
	wg         sync.WaitGroup
	recorder   record.EventRecorder
	log        *zap.SugaredLogger
}

// NewSSHStatusController 创建一个新的SSH状态控制器
func NewSSHStatusController(kubeClient kubernetes.Interface, config *config.AgentConfig, mgr ctrl.Manager) SSHStatusController {
	log.Logger.Debugf("Creating new SSHStatus controller")

	// 创建事件记录器
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(mgr.GetScheme(), corev1.EventSource{Component: "ssh-controller"})

	controller := &sshStatusController{
		client:     mgr.GetClient(),
		kubeClient: kubeClient,
		config:     config,
		stopCh:     make(chan struct{}),
		recorder:   recorder,
		log:        log.Logger.Named("sshstatus"),
	}

	log.Logger.Debugf("SSHStatus controller created successfully")
	return controller
}

// Stop 停止SSH状态控制器
func (c *sshStatusController) Stop() {
	c.log.Info("Stopping SSHStatus controller")
	close(c.stopCh)
	c.wg.Wait()
	c.log.Info("SSHStatus controller stopped successfully")
}

// SetupWithManager 设置controller-runtime manager
func (c *sshStatusController) SetupWithManager(mgr ctrl.Manager) error {
	go func() {
		<-mgr.Elected()
		c.log.Info("Elected as leader, begin to start SSH status controller")
		// 启动SSH状态的周期更新
		go c.UpdateSSHStatusAtInterval()
	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.SSHStatus{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 30, // 设置并发数量
		}).
		Complete(c)
}

// UpdateSecret 更新SSH主机的认证信息
func (c *sshStatusController) UpdateSecret(secretName, secretNamespace, username, password string) {
	c.log.Debugf("Updating secret %s/%s with username: %s", secretNamespace, secretName, username)
	// 这里可以添加更新secret的逻辑
	// 例如: 更新缓存中的认证信息或通知相关组件重新加载secret
}

// updateSubnetNameFromDhcpClientDetails 检查SSHStatus的IP是否在Subnet的dhcpClientDetails中，并更新subnetName
func (c *sshStatusController) updateSubnetNameFromDhcpClientDetails(sshStatus *topohubv1beta1.SSHStatus, logger *zap.SugaredLogger) error {
	if sshStatus == nil || sshStatus.Status.Basic.IpAddr == "" {
		return nil
	}

	targetIP := sshStatus.Status.Basic.IpAddr

	// 获取所有Subnet资源
	subnets := &topohubv1beta1.SubnetList{}
	if err := c.client.List(context.TODO(), subnets); err != nil {
		return fmt.Errorf("failed to list subnets: %v", err)
	}

	// 检查每个Subnet的dhcpClientDetails
	for _, subnet := range subnets.Items {
		if subnet.Status.DhcpClientDetails == "" {
			continue
		}

		// 解析dhcpClientDetails为map[string]interface{}，其中key是IP地址
		var dhcpClients map[string]interface{}
		if err := json.Unmarshal([]byte(subnet.Status.DhcpClientDetails), &dhcpClients); err != nil {
			logger.Warnf("Failed to unmarshal dhcpClientDetails for subnet %s: %v", subnet.Name, err)
			continue
		}

		// 检查目标IP是否在map的键中
		if _, exists := dhcpClients[targetIP]; exists {
			// 找到匹配的IP，更新subnetName
			if sshStatus.Status.Basic.SubnetName == nil || *sshStatus.Status.Basic.SubnetName != subnet.Name {
				subnetName := subnet.Name
				sshStatus.Status.Basic.SubnetName = &subnetName
				logger.Infof("Updated subnetName to %s for SSHStatus %s (IP: %s)", subnet.Name, sshStatus.Name, targetIP)

				// 更新SSHStatus资源
				if err := c.client.Status().Update(context.TODO(), sshStatus); err != nil {
					return fmt.Errorf("failed to update SSHStatus %s: %v", sshStatus.Name, err)
				}
			}
			return nil
		}
	}
	return nil
}
