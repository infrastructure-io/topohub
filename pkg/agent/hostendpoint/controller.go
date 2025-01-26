package hostendpoint

import (
	"context"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/agent/config"
	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// HostEndpointReconciler reconciles a HostEndpoint object
type HostEndpointReconciler struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
}

// NewHostEndpointReconciler creates a new HostEndpoint reconciler
func NewHostEndpointReconciler(mgr ctrl.Manager, kubeClient kubernetes.Interface, config *config.AgentConfig) (*HostEndpointReconciler, error) {
	return &HostEndpointReconciler{
		client:     mgr.GetClient(),
		kubeClient: kubeClient,
		config:     config,
	}, nil
}

// Reconcile handles the reconciliation of HostEndpoint objects
func (r *HostEndpointReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.Logger.With(
		zap.String("hostendpoint", req.Name),
	)

	// 获取 HostEndpoint
	hostEndpoint := &bmcv1beta1.HostEndpoint{}
	if err := r.client.Get(ctx, req.NamespacedName, hostEndpoint); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("HostEndpoint not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get HostEndpoint")
		return reconcile.Result{}, err
	}

	// 检查是否属于当前的 ClusterAgent
	if hostEndpoint.Spec.ClusterAgent != r.config.ClusterAgentName {
		logger.Debugf("HostEndpoint belongs to different ClusterAgent, ignoring",
			"expectedAgent", r.config.ClusterAgentName,
			"actualAgent", hostEndpoint.Spec.ClusterAgent)
		return reconcile.Result{}, nil
	}
	logger.Info("Reconciling my cluster's HostEndpoint")

	// 处理 HostEndpoint
	if err := r.handleHostEndpoint(ctx, hostEndpoint, logger); err != nil {
		logger.Error(err, "Failed to handle HostEndpoint")
		return reconcile.Result{
			RequeueAfter: time.Second * 2,
		}, err
	}

	return reconcile.Result{}, nil
}

// handleHostEndpoint 处理单个 HostEndpoint 资源
func (r *HostEndpointReconciler) handleHostEndpoint(ctx context.Context, hostEndpoint *bmcv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// Try to get existing HostStatus
	existing := &bmcv1beta1.HostStatus{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, existing)
	if err == nil {
		// HostStatus exists, check if spec changed
		if specEqual(existing.Status.Basic, hostEndpoint.Spec) {
			logger.Debugf("HostStatus %s exists with same spec, no update needed", name)
			return nil
		}

		// Spec changed, update the object
		logger.Infof("Updating HostStatus %s due to spec change", name)

		// Create a copy of the existing object to avoid modifying the cache
		updated := existing.DeepCopy()
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		updated.Status.Basic = bmcv1beta1.BasicInfo{
			Type:            bmcv1beta1.HostTypeEndpoint,
			IpAddr:          hostEndpoint.Spec.IPAddr,
			SecretName:      *hostEndpoint.Spec.SecretName,
			SecretNamespace: *hostEndpoint.Spec.SecretNamespace,
			Https:           *hostEndpoint.Spec.HTTPS,
			Port:            *hostEndpoint.Spec.Port,
		}

		if err := r.client.Update(ctx, updated); err != nil {
			if errors.IsConflict(err) {
				logger.Debugf("Conflict updating HostStatus %s, will retry", name)
				return err
			}
			logger.Errorf("Failed to update HostStatus %s: %v", name, err)
			return err
		}
		logger.Infof("Successfully updated HostStatus %s", name)
		logger.Debugf("Updated HostStatus details - IP: %s, Secret: %s/%s, Port: %d",
			updated.Status.Basic.IpAddr,
			updated.Status.Basic.SecretNamespace,
			updated.Status.Basic.SecretName,
			updated.Status.Basic.Port)
		return nil
	}

	if !errors.IsNotFound(err) {
		logger.Errorf("Failed to get HostStatus %s: %v", name, err)
		return err
	}

	// HostStatus doesn't exist, create new one
	hostStatus := &bmcv1beta1.HostStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				bmcv1beta1.LabelIPAddr:     hostEndpoint.Spec.IPAddr,
				bmcv1beta1.LabelClientMode: bmcv1beta1.HostTypeEndpoint,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         bmcv1beta1.APIVersion,
					Kind:               bmcv1beta1.KindHostEndpoint,
					Name:               hostEndpoint.Name,
					UID:                hostEndpoint.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
	}

	// HostStatus doesn't exist, create new one
	// IMPORTANT: When creating a new HostStatus, we must follow a two-step process:
	// 1. First create the resource with only metadata (no status). This is because
	//    the Kubernetes API server does not allow setting status during creation.
	// 2. Then update the status separately using UpdateStatus. If we try to set
	//    status during creation, the status will be silently ignored, leading to
	//    a HostStatus without any status information until the next reconciliation.
	logger.Debugf("Creating new HostStatus %s", name)
	if err := r.client.Create(ctx, hostStatus); err != nil {
		logger.Errorf("Failed to create HostStatus %s: %v", name, err)
		return err
	}

	// Get the latest version of the resource after creation
	// if err := r.client.Get(ctx, client.ObjectKey{Name: name}, hostStatus); err != nil {
	// 	logger.Errorf("Failed to get latest version of HostStatus %s: %v", name, err)
	// 	return err
	// }

	// Now update the status using the latest version
	hostStatus.Status = bmcv1beta1.HostStatusStatus{
		Healthy:        false,
		ClusterAgent:   hostEndpoint.Spec.ClusterAgent,
		LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
		Basic: bmcv1beta1.BasicInfo{
			Type:            bmcv1beta1.HostTypeEndpoint,
			IpAddr:          hostEndpoint.Spec.IPAddr,
			SecretName:      *hostEndpoint.Spec.SecretName,
			SecretNamespace: *hostEndpoint.Spec.SecretNamespace,
			Https:           *hostEndpoint.Spec.HTTPS,
			Port:            *hostEndpoint.Spec.Port,
		},
		Info: map[string]string{},
		Log: bmcv1beta1.LogStruct{
			TotalLogAccount:   0,
			WarningLogAccount: 0,
			LastestLog:        nil,
			LastestWarningLog: nil,
		},
	}

	if err := r.client.Status().Update(ctx, hostStatus); err != nil {
		logger.Errorf("Failed to update status of HostStatus %s: %v", name, err)
		return err
	}

	logger.Infof("Successfully created HostStatus %s", name)
	logger.Debugf("HostStatus details - IP: %s, Secret: %s/%s, Port: %d",
		hostStatus.Status.Basic.IpAddr,
		hostStatus.Status.Basic.SecretNamespace,
		hostStatus.Status.Basic.SecretName,
		hostStatus.Status.Basic.Port)
	return nil
}

// specEqual checks if the HostStatus basic info matches the HostEndpoint spec
func specEqual(basic bmcv1beta1.BasicInfo, spec bmcv1beta1.HostEndpointSpec) bool {
	return basic.IpAddr == spec.IPAddr &&
		basic.SecretName == *spec.SecretName &&
		basic.SecretNamespace == *spec.SecretNamespace &&
		basic.Https == *spec.HTTPS &&
		basic.Port == *spec.Port
}

// SetupWithManager sets up the controller with the Manager
func (r *HostEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bmcv1beta1.HostEndpoint{}).
		Complete(r)
}
