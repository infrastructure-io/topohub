package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/infrastructure-io/topohub/pkg/config"
	"github.com/infrastructure-io/topohub/pkg/hostendpoint"
	"github.com/infrastructure-io/topohub/pkg/bindingip"
	"github.com/infrastructure-io/topohub/pkg/hostoperation"
	"github.com/infrastructure-io/topohub/pkg/hoststatus"
	"github.com/infrastructure-io/topohub/pkg/httpserver"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	crdclientset "github.com/infrastructure-io/topohub/pkg/k8s/client/clientset/versioned/typed/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/secret"
	"github.com/infrastructure-io/topohub/pkg/subnet"
	hostendpointwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostendpoint"
	hostoperationwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostoperation"
	hoststatuswebhook "github.com/infrastructure-io/topohub/pkg/webhook/hoststatus"
	bindingipwebhook "github.com/infrastructure-io/topohub/pkg/webhook/bindingip"
	subnetwebhook "github.com/infrastructure-io/topohub/pkg/webhook/subnet"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(topohubv1beta1.AddToScheme(scheme))
}

func main() {
	// Parse command line flags
	probePort := flag.String("health-probe-port", "8081", "The address the probe endpoint binds to.")
	webhookPort := flag.String("webhook-port", "8082", "The address the probe endpoint binds to.")
	metricsPort := flag.String("metrics-port", "8083", "The address the metric endpoint binds to.")
	flag.Parse()

	// Initialize logger
	logLevel := os.Getenv("LOG_LEVEL")
	log.InitStdoutLogger(logLevel)

	// Set controller-runtime logger
	ctrl.SetLogger(zap.New())

	log.Logger.Info("Starting TopoHub")

	// Initialize Kubernetes clients
	k8sClient, _, err := initClients()
	if err != nil {
		log.Logger.Errorf("Failed to initialize clients: %v", err)
		os.Exit(1)
	}

	// Load agent configuration
	agentConfig, err := config.LoadAgentConfig()
	if err != nil {
		log.Logger.Errorf("Failed to load agent configuration: %v", err)
		os.Exit(1)
	}

	log.Logger.Info("configuration loaded and validated successfully")
	log.Logger.Debugf("configuration details: %+v", agentConfig)

	// Create manager
	webhookPortInt, err := strconv.Atoi(*webhookPort)
	if err != nil {
		log.Logger.Errorf("Failed to convert webhook port to int: %v", err)
		os.Exit(1)
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: ":" + *metricsPort,
		},
		HealthProbeBindAddress: ":" + *probePort,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: webhookPortInt,
			//CertDir: agentConfig.WebhookCertDir,
		}),
		LeaderElection:          true,
		LeaderElectionID:        "topohub-lock",
		LeaderElectionNamespace: agentConfig.PodNamespace,
	})
	if err != nil {
		log.Logger.Errorf("Unable to start manager: %v", err)
		os.Exit(1)
	}

	// Setup HostEndpoint webhook
	if err = (&hostendpointwebhook.HostEndpointWebhook{}).SetupWebhookWithManager(mgr, *agentConfig); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "HostEndpoint", err)
		os.Exit(1)
	}

	// Setup HostOperation webhook
	if err = (&hostoperationwebhook.HostOperationWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "HostOperation", err)
		os.Exit(1)
	}

	// Setup Subnet webhook
	if err = (&subnetwebhook.SubnetWebhook{}).SetupWebhookWithManager(mgr, *agentConfig); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "DhcpSubnet", err)
		os.Exit(1)
	}

	// setup binding ip webhook
	if err = (&bindingipwebhook.BindingIPWebhook{}).SetupWebhookWithManager(mgr, *agentConfig); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "BindingIp", err)
		os.Exit(1)
	}

	// Setup HostStatus webhook
	if err = (&hoststatuswebhook.HostStatusWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "HostStatus", err)
		os.Exit(1)
	}

	// todo: subnet manager
	subnetMgr := subnet.NewSubnetReconciler(*agentConfig, k8sClient)
	if err = subnetMgr.SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("Failed to setup subnet manager: %v", err)
		os.Exit(1)
	}
	addDhcpChan, deleteDhcpChan := subnetMgr.GetDhcpClientEvents()
	deleteHostStatusChan := subnetMgr.GetHostStatusEvents()
	// Initialize hoststatus controller
	hostStatusCtrl := hoststatus.NewHostStatusController(k8sClient, agentConfig, mgr, addDhcpChan, deleteDhcpChan, deleteHostStatusChan)
	if err = hostStatusCtrl.SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("Unable to create hoststatus controller: %v", err)
		os.Exit(1)
	}

	// initialize secret controller
	secretCtrl, err := secret.NewSecretReconciler(mgr, agentConfig, hostStatusCtrl)
	if err != nil {
		log.Logger.Errorf("Failed to create secret controller: %v", err)
		os.Exit(1)
	}
	if err = secretCtrl.SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("Unable to create secret controller: %v", err)
		os.Exit(1)
	}

	// Initialize hostendpoint controller, it will watch the hostendpoint and update the hoststatus
	hostEndpointCtrl, err := hostendpoint.NewHostEndpointReconciler(mgr, k8sClient, agentConfig)
	if err != nil {
		log.Logger.Errorf("Failed to create hostendpoint controller: %v", err)
		os.Exit(1)
	}
	if err = hostEndpointCtrl.SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("Unable to create hostendpoint controller: %v", err)
		os.Exit(1)
	}

	// Initialize hostoperation controller
	hostOperationCtrl, err := hostoperation.NewHostOperationController(mgr, agentConfig)
	if err != nil {
		log.Logger.Errorf("Failed to create hostoperation controller: %v", err)
		os.Exit(1)
	}

	if err = hostOperationCtrl.SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("Unable to create hostoperation controller: %v", err)
		os.Exit(1)
	}

	// Initialize bindingIP controller
	bindingIPCtrl := bindingip.NewBindingIPController(mgr, agentConfig)
	if err != nil {
		log.Logger.Errorf("Failed to create bindingip controller: %v", err)
		os.Exit(1)
	}
	if err = bindingIPCtrl.SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("Unable to create bindingip controller: %v", err)
		os.Exit(1)
	}


	// start http server for pxe and ztp
	if agentConfig.HttpEnabled {
		log.Logger.Info("Http server is enabled for pxe and ztp")
		httpServer := httpserver.NewHttpServer(*agentConfig)
		httpServer.Run()
	} else {
		log.Logger.Info("Http server is disabled for pxe and ztp")
	}

	// Add health check
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Logger.Errorf("Unable to set up health check: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Logger.Errorf("Unable to set up ready check: %v", err)
		os.Exit(1)
	}

	// Create context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start manager
	go func() {
		log.Logger.Info("Starting manager")
		// If LeaderElection is used, the binary must be exited immediately after this returns,
		// otherwise components that need leader election might continue to run after the leader
		// lock was lost.
		if err := mgr.Start(ctx); err != nil {
			log.Logger.Errorf("Problem running manager: %v", err)

			// Stop DHCP server to remove ip if it was started

			os.Exit(1)
		}
	}()

	go func() {
		log.Logger.Infof("waiting for leader elected")
		<-mgr.Elected()
		log.Logger.Infof("I am elected as the Leader")
	}()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Main loop - sleep and log periodically
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Logger.Debug("Agent still running...")

		case sig := <-sigChan:
			log.Logger.Infof("Received signal %v, shutting down...", sig)

			// Stop DHCP server to remove ip if it was started

			// Stop hoststatus controller
			hostStatusCtrl.Stop()

			// Cancel context to stop manager
			cancel()

			return
		}
	}
}

// initClients initializes Kubernetes clients
func initClients() (*kubernetes.Clientset, *crdclientset.TopohubV1beta1Client, error) {
	var config *rest.Config
	var err error

	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	runtimeClient, err := crdclientset.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return clientset, runtimeClient, nil
}
