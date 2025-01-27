package hoststatus

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/config"
	hoststatusdata "github.com/infrastructure-io/topohub/pkg/hoststatus/data"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/subnet/dhcpserver"
)

type HostStatusController interface {
	// 把 dhcp 的 client 事件 channel 返给外部，以供外部来通知自己
	GetDHCPEventChan() (chan<- dhcpserver.DhcpClientInfo, chan<- dhcpserver.DhcpClientInfo)
	Stop()
	SetupWithManager(ctrl.Manager) error
	// 更新 bmc 主机的 认证信息
	UpdateSecret(string, string, string, string)
}

type hostStatusController struct {
	client     client.Client
	kubeClient kubernetes.Interface
	// config holds the agent configuration, which is used to
	// determine the cluster agent name and the path to the feature
	// configuration directory.
	config     *config.AgentConfig
	addChan    chan dhcpserver.DhcpClientInfo
	deleteChan chan dhcpserver.DhcpClientInfo
	stopCh     chan struct{}
	wg         sync.WaitGroup
	recorder   record.EventRecorder
}

func NewHostStatusController(kubeClient kubernetes.Interface, config *config.AgentConfig, mgr ctrl.Manager) HostStatusController {
	log.Logger.Debugf("Creating new HostStatus controller")

	// Create event recorder
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(mgr.GetScheme(), corev1.EventSource{Component: "bmc-controller"})

	controller := &hostStatusController{
		client:     mgr.GetClient(),
		kubeClient: kubeClient,
		config:     config,
		addChan:    make(chan dhcpserver.DhcpClientInfo),
		deleteChan: make(chan dhcpserver.DhcpClientInfo),
		stopCh:     make(chan struct{}),
		recorder:   recorder,
	}

	log.Logger.Debugf("HostStatus controller created successfully")
	return controller
}

func (c *hostStatusController) Stop() {
	log.Logger.Info("Stopping HostStatus controller")
	close(c.stopCh)
	c.wg.Wait()
	log.Logger.Info("HostStatus controller stopped successfully")
}

func (c *hostStatusController) GetDHCPEventChan() (chan<- dhcpserver.DhcpClientInfo, chan<- dhcpserver.DhcpClientInfo) {
	return c.addChan, c.deleteChan
}

// SetupWithManager 设置 controller-runtime manager
func (c *hostStatusController) SetupWithManager(mgr ctrl.Manager) error {

	go func() {
		select {
		case <-mgr.Elected():
			log.Logger.Info("Elected as leader, begin to start all controllers")
			// 启动 DHCP 事件处理
			go c.processDHCPEvents()
			// 启动 hoststatus spec.info 的	周期更新
			go c.UpdateHostStatusAtInterval()
		}
	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.HostStatus{}).
		Complete(c)
}

func (c *hostStatusController) UpdateSecret(secretName, secretNamespace, username, password string) {
	if secretName == c.config.RedfishSecretName && secretNamespace == c.config.RedfishSecretNamespace {
		log.Logger.Info("update default secret")
	}

	log.Logger.Debugf("updating secet in cache for secret %s/%s", secretNamespace, secretName)
	changedHosts := hoststatusdata.HostCacheDatabase.UpdateSecet(secretName, secretNamespace, username, password)
	for _, name := range changedHosts {
		log.Logger.Infof("update hostStatus %s after secret is changed", name)
		if err := c.UpdateHostStatusInfoWrapper(name); err != nil {
			log.Logger.Errorf("Failed to update host status: %v", err)
		}
	}

}
