package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/infrastructure-io/topohub/cmd/topohub/cmmanager"
	"github.com/infrastructure-io/topohub/cmd/topohub/options"
	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	"github.com/infrastructure-io/topohub/pkg/clients/ssh"
	"github.com/infrastructure-io/topohub/pkg/config"
	"github.com/infrastructure-io/topohub/pkg/debug"
	"github.com/infrastructure-io/topohub/pkg/httpserver"
	crdclientset "github.com/infrastructure-io/topohub/pkg/k8s/client/clientset/versioned/typed/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

func main() {
	var opts options.TopohubFlags
	options.ParseFlags(&opts)

	// Create context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logger
	logLevel := os.Getenv("LOG_LEVEL")
	log.InitStdoutLogger(log.LogLevel(logLevel))
	// Set controller-runtime logger
	ctrl.SetLogger(zap.New())

	log.Logger.Info("Starting topohub")

	enableDebugs(&opts)

	// Load agent configuration
	agentConfig, err := config.LoadAgentConfig()
	if err != nil {
		log.Logger.Errorf("Failed to load agent configuration: %v", err)
		os.Exit(1)
	}
	log.Logger.Info("Configuration loaded and validated successfully")
	log.Logger.Debugf("Configuration details: %+v", agentConfig)

	// Init session pools
	redfish.InitSessionPool(ctx)
	ssh.InitSessionPool(ctx)

	// Initialize Kubernetes clients
	k8scli, _, err := initClients()
	if err != nil {
		log.Logger.Errorf("Failed to initialize clients, err: %v", err)
		os.Exit(1)
	}
	// Create manager
	mgr, err := cmmanager.NewControllerManager(&opts)
	if err != nil {
		log.Logger.Errorf("Failed to create manager, err: %v", err)
		os.Exit(1)
	}

	stopFns, err := cmmanager.RegisterControllers(mgr, k8scli, agentConfig)
	if err != nil {
		log.Logger.Error(err)
		os.Exit(1)
	}

	cmmanager.StartControllers(ctx, mgr)
	startHttpServer(agentConfig)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Main loop - sleep and log periodically
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Graceful shutdown and health Check
	for {
		select {
		case <-ticker.C:
			log.Logger.Debug("Topohub still running...")
		case sig := <-sigChan:
			log.Logger.Infof("Received signal %v, shutting down...", sig)
			// TODO: Stop DHCP server to remove ip if it was started

			// Call stop functions that needs to stop the controller
			for _, stopFn := range stopFns {
				stopFn()
			}

			// Cancel context to stop manager
			cancel()

			return
		}
	}
}

func enableDebugs(opts *options.TopohubFlags) {
	// start pprof server
	debug.RunPProf(opts.PprofAddress, opts.PprofPort)
	// start pyroscope server
	debug.RunPyroscope(opts.PyroscopeAddress, opts.PyroscopeTag)
}

// initClients initializes Kubernetes clients
func initClients() (kubernetes.Interface, crdclientset.TopohubV1beta1Interface, error) {
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

// start http server for pxe and ztp
func startHttpServer(agentConfig *config.AgentConfig) {
	if agentConfig.HttpEnabled {
		log.Logger.Info("Http server is enabled for pxe and ztp")
		httpServer := httpserver.NewHttpServer(agentConfig)
		httpServer.Run()
	} else {
		log.Logger.Info("Http server is disabled for pxe and ztp")
	}
}
