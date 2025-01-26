package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/infrastructure-io/topohub/pkg/agent/config"
	"github.com/infrastructure-io/topohub/pkg/agent/hostendpoint"
	"github.com/infrastructure-io/topohub/pkg/agent/hostoperation"
	"github.com/infrastructure-io/topohub/pkg/agent/hoststatus"
	secretcontroller "github.com/infrastructure-io/topohub/pkg/agent/secret"
	"github.com/infrastructure-io/topohub/pkg/dhcpserver"
	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	crdclientset "github.com/infrastructure-io/topohub/pkg/k8s/client/clientset/versioned/typed/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(bmcv1beta1.AddToScheme(scheme))
}

func main() {
	// Parse command line flags
	metricsAddr := flag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	probeAddr := flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.Parse()

	// Initialize logger
	logLevel := os.Getenv("LOG_LEVEL")
	log.InitStdoutLogger(logLevel)

	// Set controller-runtime logger
	ctrl.SetLogger(zap.New())

	log.Logger.Info("Starting BMC agent")

	// Initialize Kubernetes clients
	k8sClient, _, err := initClients()
	if err != nil {
		log.Logger.Errorf("Failed to initialize clients: %v", err)
		os.Exit(1)
	}

	// Load agent configuration
	agentConfig, err := config.LoadAgentConfig(k8sClient)
	if err != nil {
		log.Logger.Errorf("Failed to load agent configuration: %v", err)
		os.Exit(1)
	}

	log.Logger.Info("Agent configuration loaded and validated successfully")
	log.Logger.Debug("Agent configuration details:")
	log.Logger.Debugf("\n%s", agentConfig.GetDetailString())

	// Create manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: *metricsAddr,
		},
		HealthProbeBindAddress: *probeAddr,
	})
	if err != nil {
		log.Logger.Errorf("Unable to start manager: %v", err)
		os.Exit(1)
	}

	// Initialize hoststatus controller
	hostStatusCtrl := hoststatus.NewHostStatusController(k8sClient, agentConfig, mgr)

	if err = hostStatusCtrl.SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("Unable to create hoststatus controller: %v", err)
		os.Exit(1)
	}

	// initialize secret controller
	secretCtrl, err := secretcontroller.NewSecretReconciler(mgr, agentConfig, hostStatusCtrl)
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

	// Get DHCP event channels for hoststatus
	addChan, deleteChan := hostStatusCtrl.GetDHCPEventChan()

	// Initialize DHCP server if enabled
	var dhcpSrv dhcpserver.DhcpServer
	if agentConfig.AgentObjSpec.Feature.EnableDhcpServer {
		log.Logger.Info("Starting DHCP server...")
		var err error
		dhcpSrv, err = dhcpserver.NewDhcpServer(
			agentConfig.AgentObjSpec.Feature.DhcpServerConfig,
			agentConfig.ClusterAgentName,
			addChan,
			deleteChan,
		)
		if err != nil {
			log.Logger.Errorf("Failed to initialize DHCP server: %v", err)
			os.Exit(1)
		}

		if err := dhcpSrv.Start(); err != nil {
			log.Logger.Errorf("Failed to start DHCP server: %v", err)
			os.Exit(1)
		}
		log.Logger.Info("DHCP server started successfully")
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
			if dhcpSrv != nil {
				log.Logger.Info("Stopping DHCP server...")
				if err := dhcpSrv.Stop(); err != nil {
					log.Logger.Errorf("Error stopping DHCP server: %v", err)
				}
			}

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
