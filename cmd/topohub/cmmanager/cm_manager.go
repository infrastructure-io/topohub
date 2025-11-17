package cmmanager

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/infrastructure-io/topohub/cmd/topohub/options"
	"github.com/infrastructure-io/topohub/pkg/bindingip"
	"github.com/infrastructure-io/topohub/pkg/config"
	"github.com/infrastructure-io/topohub/pkg/hostendpoint"
	"github.com/infrastructure-io/topohub/pkg/hostoperation"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/redfishstatus"
	"github.com/infrastructure-io/topohub/pkg/secret"
	"github.com/infrastructure-io/topohub/pkg/sshstatus"
	"github.com/infrastructure-io/topohub/pkg/subnet"
	bindingipwebhook "github.com/infrastructure-io/topohub/pkg/webhook/bindingip"
	hostendpointwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostendpoint"
	hostoperationwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostoperation"
	redfishstatuswebhook "github.com/infrastructure-io/topohub/pkg/webhook/redfishstatus"
	sshstatuswebhook "github.com/infrastructure-io/topohub/pkg/webhook/sshstatus"
	subnetwebhook "github.com/infrastructure-io/topohub/pkg/webhook/subnet"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(topohubv1beta1.AddToScheme(scheme))
}

type StopFn func()

func NewControllerManager(opts *options.TopohubFlags) (manager.Manager, error) {
	webhookPortInt, err := strconv.Atoi(opts.WebhookPort)
	if err != nil {
		return nil, fmt.Errorf("failed to convert webhook port to int, err: %v", err)
	}
	return ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: ":" + opts.MetricsPort,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: webhookPortInt,
			// CertDir: agentConfig.WebhookCertDir,
		}),
		// Leader Election disabled for single pod deployment
		LeaderElection: false,
		// Strip ManagedFields from all cached objects to reduce memory usage
		// This prevents ManagedFields from accumulating in the informer cache
		Cache: cache.Options{
			DefaultTransform: cache.TransformStripManagedFields(),
		},
	})
}

func RegisterControllers(mgr manager.Manager, k8scli kubernetes.Interface, agentConfig *config.AgentConfig) ([]StopFn, error) {
	stopFns := make([]StopFn, 0, 2)
	// Setup HostEndpoint webhook
	if err := (&hostendpointwebhook.HostEndpointWebhook{}).SetupWebhookWithManager(mgr, *agentConfig); err != nil {
		return nil, fmt.Errorf("unable to create webhook %s, err: %v", "HostEndpoint", err)
	}
	// Setup HostOperation webhook
	if err := (&hostoperationwebhook.HostOperationWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create webhook %s, err: %v", "HostOperation", err)
	}
	// Setup Subnet webhook
	if err := (&subnetwebhook.SubnetWebhook{}).SetupWebhookWithManager(mgr, *agentConfig); err != nil {
		return nil, fmt.Errorf("unable to create webhook %s. err: %v", "DhcpSubnet", err)
	}
	// setup binding ip webhook
	if err := (&bindingipwebhook.BindingIPWebhook{}).SetupWebhookWithManager(mgr, *agentConfig); err != nil {
		return nil, fmt.Errorf("unable to create webhook %s, err: %v", "BindingIp", err)
	}
	// Setup RedfishStatus webhook
	if err := redfishstatuswebhook.SetupWebhookWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to setup redfishstatus webhook. err: %v", err)
	}
	// Setup SSHStatus webhook
	if err := (&sshstatuswebhook.SSHStatusWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create webhook %s, err: %v", "SSHStatus", err)
	}

	// todo: subnet manager
	subnetMgr := subnet.NewSubnetReconciler(*agentConfig, k8scli)
	if err := subnetMgr.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("failed to setup subnet manager, err: %v", err)
	}

	// dhcp client events for redfishstatus
	addDhcpChan, deleteDhcpChan := subnetMgr.GetDhcpClientEventsForRedfishStatus()
	addBindingIpChan, deleteBindingIpChan := subnetMgr.GetBindingIpEvents()
	// Initialize redfishstatus controller
	redfishStatusCtrl := redfishstatus.NewRedfishStatusController(k8scli, agentConfig, mgr, addDhcpChan, deleteDhcpChan)
	if err := redfishStatusCtrl.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create redfishstatus controller, err: %v", err)
	}
	stopFns = append(stopFns, redfishStatusCtrl.Stop)

	// Initialize secret controller
	secretCtrl, err := secret.NewSecretReconciler(mgr)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret controller, err: %v", err)
	}
	if err := secretCtrl.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create secret controller, err: %v", err)
	}
	// Initialize hostoperation controller
	hostOperationCtrl, err := hostoperation.NewHostOperationController(mgr, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create hostoperation controller, err: %v", err)
	}

	if err := hostOperationCtrl.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create hostoperation controller, err: %v", err)
	}

	// Initialize sshstatus controller
	sshStatusCtrl := sshstatus.NewSSHStatusController(k8scli, agentConfig, mgr)
	if err := sshStatusCtrl.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create sshstatus controller, err: %v", err)
	}
	stopFns = append(stopFns, sshStatusCtrl.Stop)

	// Initialize hostendpoint controller, it will watch the hostendpoint and update the redfishstatus/sshstatus
	hostEndpointCtrl := hostendpoint.NewHostEndpointReconciler(mgr, redfishStatusCtrl, sshStatusCtrl)
	if err := hostEndpointCtrl.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create hostendpoint controller, err: %v", err)
	}

	// Initialize bindingIP controller
	bindingIPCtrl := bindingip.NewBindingIPController(mgr, agentConfig, addBindingIpChan, deleteBindingIpChan)
	if err := bindingIPCtrl.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create bindingip controller, err: %v", err)
	}

	// Add health check
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up health check, err: %v", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up ready check, err: %v", err)
	}
	return stopFns, nil
}

func StartControllers(ctx context.Context, mgr manager.Manager) {
	// Start manager
	go func() {
		log.Logger.Info("Starting manager")
		if err := mgr.Start(ctx); err != nil {
			log.Logger.Errorf("Problem running manager, err: %v", err)
			// Stop DHCP server to remove ip if it was started
			os.Exit(1)
		}
	}()
}
