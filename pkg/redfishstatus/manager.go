package redfishstatus

import (
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
	redfishstatusdata "github.com/infrastructure-io/topohub/pkg/redfishstatus/data"
	"github.com/infrastructure-io/topohub/pkg/subnet/dhcpserver"
)

type RedfishStatusController interface {
	Stop()
	SetupWithManager(ctrl.Manager) error
	// 更新 bmc 主机的 认证信息
	UpdateSecret(string, string, string, string)
}

type redfishStatusController struct {
	client     client.Client
	kubeClient kubernetes.Interface
	// config holds the agent configuration, which is used to
	// determine the cluster agent name and the path to the feature
	// configuration directory.
	config     *config.AgentConfig
	stopCh     chan struct{}
	wg         sync.WaitGroup
	recorder   record.EventRecorder
	addChan    chan dhcpserver.DhcpClientInfo
	deleteChan chan dhcpserver.DhcpClientInfo

	log *zap.SugaredLogger
}

func NewRedfishStatusController(kubeClient kubernetes.Interface, config *config.AgentConfig, mgr ctrl.Manager, addChan, deleteChan chan dhcpserver.DhcpClientInfo) RedfishStatusController {
	log.Logger.Debugf("Creating new RedfishStatus controller")

	// Create event recorder
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(mgr.GetScheme(), corev1.EventSource{Component: "bmc-controller"})

	controller := &redfishStatusController{
		client:     mgr.GetClient(),
		kubeClient: kubeClient,
		config:     config,
		addChan:    addChan,
		deleteChan: deleteChan,
		stopCh:     make(chan struct{}),
		recorder:   recorder,
		log:        log.Logger.Named("redfishstatus"),
	}

	log.Logger.Debugf("RedfishStatus controller created successfully")
	return controller
}

func (c *redfishStatusController) Stop() {
	c.log.Info("Stopping RedfishStatus controller")
	close(c.stopCh)
	c.wg.Wait()
	c.log.Info("RedfishStatus controller stopped successfully")
}

// SetupWithManager 设置 controller-runtime manager
func (c *redfishStatusController) SetupWithManager(mgr ctrl.Manager) error {

	go func() {
		<-mgr.Elected()
		c.log.Info("Elected as leader, begin to start all controllers")
		// 启动 DHCP 事件处理
		go c.processDHCPEvents()
		// 启动 redfishstatus spec.info 的	周期更新
		go c.UpdateRedfishStatusAtInterval()
	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.RedfishStatus{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 30, // 设置你希望的并发数量
		}).
		Complete(c)
}

func (c *redfishStatusController) UpdateSecret(secretName, secretNamespace, username, password string) {
	if secretName == c.config.RedfishSecretName && secretNamespace == c.config.RedfishSecretNamespace {
		c.log.Info("update default secret")
	}

	c.log.Debugf("updating secet in cache for secret %s/%s", secretNamespace, secretName)
	changedHosts := redfishstatusdata.RedfishCacheDatabase.UpdateSecet(secretName, secretNamespace, username, password)
	for _, name := range changedHosts {
		c.log.Infof("update redfishStatus %s after secret is changed", name)
		if err := c.UpdateRedfishStatusInfoWrapper(name); err != nil {
			c.log.Errorf("Failed to update redfish status: %v", err)
		}
	}
}
