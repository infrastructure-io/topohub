package sshstatus

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
)

// SSHStatusController interface defines the public methods for the SSH status controller
type SSHStatusController interface {
	Stop()
	SetupWithManager(ctrl.Manager) error
	// Update authentication information for SSH hosts
	UpdateSecret(string, string, string, string)
}

// sshStatusController implements the SSHStatusController interface
type sshStatusController struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
	stopCh     chan struct{}
	wg         sync.WaitGroup
	recorder   record.EventRecorder
	log        *zap.SugaredLogger
}

// NewSSHStatusController creates a new SSH status controller
func NewSSHStatusController(kubeClient kubernetes.Interface, config *config.AgentConfig, mgr ctrl.Manager) SSHStatusController {
	log.Logger.Debugf("Creating new SSHStatus controller")

	// Create event recorder
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

// Stop shuts down the SSH status controller
func (c *sshStatusController) Stop() {
	c.log.Info("Stopping SSHStatus controller")
	close(c.stopCh)
	c.wg.Wait()
	c.log.Info("SSHStatus controller stopped successfully")
}

// SetupWithManager sets up the controller with the manager
func (c *sshStatusController) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: 注释掉可能导致内存泄漏的 goroutine
	// go func() {
	// 	// <-mgr.Elected()
	// 	// c.log.Info("Elected as leader, begin to start SSH status controller")
	// 	// Start periodic SSH status updates
	// 	go c.UpdateSSHStatusAtInterval()
	// }()

	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.SSHStatus{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5, // Set the number of concurrent reconciles
		}).
		Complete(c)
}

// UpdateSecret updates the authentication information for SSH hosts
func (c *sshStatusController) UpdateSecret(secretName, secretNamespace, username, password string) {
	c.log.Debugf("Updating secret %s/%s with username: %s", secretNamespace, secretName, username)

}
