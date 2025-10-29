package redfishstatus

import (
	"sync"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/subnet/dhcpserver"
)

type RedfishStatusController interface {
	Stop()
	SetupWithManager(ctrl.Manager) error
	// update bmc host's authentication information
	UpdateSecret(string, string, string, string)
	// update RedfishStatus info field
	UpdateRedfishStatusInfo(*topohubv1beta1.RedfishStatus) error
}

type redfishStatusController struct {
	client      client.Client
	cacheReader client.Reader
	kubeClient  kubernetes.Interface
	// config holds the agent configuration, which is used to
	// determine the cluster agent name and the path to the feature
	// configuration directory.
	config      *config.AgentConfig
	stopCh      chan struct{}
	wg          sync.WaitGroup
	recorder    record.EventRecorder
	addChan     chan dhcpserver.DhcpClientInfo
	deleteChan  chan dhcpserver.DhcpClientInfo
	antsPool    *ants.Pool
	redfishPool pool.SessionPool[redfish.Client]
	log         *zap.SugaredLogger
	// infoObjPool used to cache the map object that needs to map redfish information
	infoObjPool sync.Pool
}

func NewRedfishStatusController(kubeClient kubernetes.Interface, config *config.AgentConfig, mgr ctrl.Manager, addChan, deleteChan chan dhcpserver.DhcpClientInfo) RedfishStatusController {
	log.Logger.Debugf("Creating new RedfishStatus controller")

	// Create event recorder
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(mgr.GetScheme(), corev1.EventSource{Component: "bmc-controller"})

	// Create ants goroutine pool with capacity of 100 workers
	antsPool, err := ants.NewPool(100)
	if err != nil {
		log.Logger.Errorf("Failed to create goroutine pool: %v", err)
	}

	controller := &redfishStatusController{
		client:      mgr.GetClient(),
		cacheReader: mgr.GetCache(),
		kubeClient:  kubeClient,
		config:      config,
		addChan:     addChan,
		deleteChan:  deleteChan,
		stopCh:      make(chan struct{}),
		recorder:    recorder,
		antsPool:    antsPool,
		redfishPool: redfish.GetSessionPool(),
		log:         log.Logger.Named("redfishstatus"),
		infoObjPool: sync.Pool{
			New: func() any {
				return make(map[string]string)
			},
		},
	}

	log.Logger.Debugf("RedfishStatus controller created successfully")
	return controller
}

func (c *redfishStatusController) Stop() {
	c.log.Info("Stopping RedfishStatus controller")
	close(c.stopCh)
	c.wg.Wait()

	// Release the ants pool if it exists
	if c.antsPool != nil {
		c.log.Info("Releasing ants goroutine pool")
		c.antsPool.Release()
		c.antsPool = nil
	}

	c.log.Info("RedfishStatus controller stopped successfully")
}

// SetupWithManager setup controller-runtime manager
func (c *redfishStatusController) SetupWithManager(mgr ctrl.Manager) error {
	go func() {
		<-mgr.Elected()
		c.log.Info("Elected as leader, begin to start all controllers")
		// 启动 DHCP 事件处理
		// go c.processDHCPEvents()
		// 启动 redfishstatus spec.info 的	周期更新
		//go c.UpdateRedfishStatusAtInterval()
	}()

	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.RedfishStatus{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 30, // 设置你希望的并发数量
		}).
		Complete(c)
}

func (c *redfishStatusController) UpdateSecret(secretName, secretNamespace, username, password string) {
}
