package hoststatus

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/infrastructure-io/topohub/pkg/agent/config"
	hoststatusdata "github.com/infrastructure-io/topohub/pkg/agent/hoststatus/data"
	"github.com/infrastructure-io/topohub/pkg/dhcpserver/types"
	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

type HostStatusController interface {
	GetDHCPEventChan() (chan<- types.ClientInfo, chan<- types.ClientInfo)
	Stop()
	SetupWithManager(mgr ctrl.Manager) error
	UpdateSecret(string, string, string, string)
}

type hostStatusController struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
	addChan    chan types.ClientInfo
	deleteChan chan types.ClientInfo
	stopCh     chan struct{}
	wg         sync.WaitGroup
	recorder   record.EventRecorder
}

func NewHostStatusController(kubeClient kubernetes.Interface, config *config.AgentConfig, mgr ctrl.Manager) HostStatusController {
	log.Logger.Debugf("Creating new HostStatus controller for cluster agent: %s", config.ClusterAgentName)

	// Create event recorder
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(mgr.GetScheme(), corev1.EventSource{Component: "bmc-controller"})

	controller := &hostStatusController{
		client:     mgr.GetClient(),
		kubeClient: kubeClient,
		config:     config,
		addChan:    make(chan types.ClientInfo),
		deleteChan: make(chan types.ClientInfo),
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

func (c *hostStatusController) GetDHCPEventChan() (chan<- types.ClientInfo, chan<- types.ClientInfo) {
	return c.addChan, c.deleteChan
}

// SetupWithManager 设置 controller-runtime manager
func (c *hostStatusController) SetupWithManager(mgr ctrl.Manager) error {

	// 启动 DHCP 事件处理
	go c.processDHCPEvents()

	// 启动 hoststatus spec.info 的	周期更新
	go c.UpdateHostStatusAtInterval()

	return ctrl.NewControllerManagedBy(mgr).
		For(&bmcv1beta1.HostStatus{}).
		Complete(c)
}

func (c *hostStatusController) UpdateSecret(secretName, secretNamespace, username, password string) {
	if secretName == c.config.AgentObjSpec.Endpoint.SecretName && secretNamespace == c.config.AgentObjSpec.Endpoint.SecretNamespace {
		log.Logger.Info("update default secret")
		// update the default secret
		c.config.Username = username
		c.config.Password = password
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
