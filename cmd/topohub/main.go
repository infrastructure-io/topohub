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
	"github.com/infrastructure-io/topohub/pkg/hostoperation"
	"github.com/infrastructure-io/topohub/pkg/hoststatus"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	crdclientset "github.com/infrastructure-io/topohub/pkg/k8s/client/clientset/versioned/typed/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/secret"
	hostendpointwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostendpoint"
	hostoperationwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostoperation"
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
	metricsPort := flag.String("metrics-port", "8080", "The address the metric endpoint binds to.")
	probePort := flag.String("health-probe-port", "8081", "The address the probe endpoint binds to.")
	webhookPort := flag.String("webhook-port", "8082", "The address the probe endpoint binds to.")
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
	log.Logger.Debug("configuration details: %+v", agentConfig)

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
			Port:    webhookPortInt,
			CertDir: agentConfig.WebhookCertDir,
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

	// Setup DhcpSubnet webhook
	if err = (&subnetwebhook.SubnetWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "DhcpSubnet", err)
		os.Exit(1)
	}

	// Initialize hoststatus controller
	hostStatusCtrl := hoststatus.NewHostStatusController(k8sClient, agentConfig, mgr)

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

	// todo: subnet manager

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
		if err := mgr.Start(ctx); err != nil {
			log.Logger.Errorf("Problem running manager: %v", err)
			os.Exit(1)
		}
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

			// Stop DHCP server if it was started

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
