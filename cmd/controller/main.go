package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	controller "github.com/infrastructure-io/topohub/pkg/controller/clusteragent"
	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	versioned "github.com/infrastructure-io/topohub/pkg/k8s/client/clientset/versioned"
	"github.com/infrastructure-io/topohub/pkg/log"
	clusteragentwebhook "github.com/infrastructure-io/topohub/pkg/webhook/clusteragent"
	hostendpointwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostendpoint"
	hostoperationwebhook "github.com/infrastructure-io/topohub/pkg/webhook/hostoperation"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(bmcv1beta1.AddToScheme(scheme))
}

func main() {
	// Get log level from environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	log.InitStdoutLogger(logLevel)

	// Set controller-runtime logger
	ctrl.SetLogger(zap.New())

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookPort int
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.IntVar(&webhookPort, "webhook-port", 443, "The port that the webhook server serves at.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.Parse()

	// 获取 POD_NAMESPACE
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		log.Logger.Error("POD_NAMESPACE environment variable not set")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: webhookPort,
		}),
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "bmc-controller-lock",
		LeaderElectionNamespace: namespace,
	})
	if err != nil {
		log.Logger.Errorf("unable to start manager: %v", err)
		os.Exit(1)
	}

	// 初始化 bmc client
	bmcClient, err := versioned.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Logger.Errorf("unable to create bmc client: %v", err)
		os.Exit(1)
	}

	if err = (&controller.ClusterAgentReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		BmcClient: bmcClient,
	}).SetupWithManager(mgr); err != nil {
		log.Logger.Errorf("unable to create controller %s: %v", "ClusterAgent", err)
		os.Exit(1)
	}

	// Setup webhook
	if err = (&clusteragentwebhook.ClusterAgentWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "ClusterAgent", err)
		os.Exit(1)
	}

	// Setup HostEndpoint webhook
	if err = (&hostendpointwebhook.HostEndpointWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "HostEndpoint", err)
		os.Exit(1)
	}

	// Setup HostOperation webhook
	if err = (&hostoperationwebhook.HostOperationWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		log.Logger.Errorf("unable to create webhook %s: %v", "HostOperation", err)
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Logger.Errorf("unable to set up health check: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Logger.Errorf("unable to set up ready check: %v", err)
		os.Exit(1)
	}

	log.Logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Logger.Errorf("problem running manager: %v", err)
		os.Exit(1)
	}
}
