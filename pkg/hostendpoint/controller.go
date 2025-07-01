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

	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// HostEndpointReconciler reconciles a HostEndpoint object
type HostEndpointReconciler struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
	log        *zap.SugaredLogger
}

// NewHostEndpointReconciler creates a new HostEndpoint reconciler
func NewHostEndpointReconciler(mgr ctrl.Manager, kubeClient kubernetes.Interface, config *config.AgentConfig) (*HostEndpointReconciler, error) {
	return &HostEndpointReconciler{
		client:     mgr.GetClient(),
		kubeClient: kubeClient,
		config:     config,
		log:        log.Logger.Named("hostendpointReconcile"),
	}, nil
}

// 只有 leader 才会执行 Reconcile
// Reconcile handles the reconciliation of HostEndpoint objects
func (r *HostEndpointReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With("hostendpoint", req.Name)

	// 获取 HostEndpoint
	hostEndpoint := &topohubv1beta1.HostEndpoint{}
	if err := r.client.Get(ctx, req.NamespacedName, hostEndpoint); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("HostEndpoint not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get HostEndpoint")
		return reconcile.Result{}, err
	}

	// 处理 HostEndpoint
	if err := r.handleHostEndpoint(ctx, hostEndpoint, logger); err != nil {
		logger.Error(err, "Failed to handle HostEndpoint")
		return reconcile.Result{
			RequeueAfter: time.Second * 2,
		}, err
	}

	return reconcile.Result{}, nil
}

// 根据 HostEndpoint ，同步更新对应的 RedfishStatus 或 SSHStatus
func (r *HostEndpointReconciler) handleHostEndpoint(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// 获取 HostEndpoint 的类型，默认为 redfish
	endpointType := topohubv1beta1.EndpointTypeRedfish
	if hostEndpoint.Spec.Type != nil {
		endpointType = *hostEndpoint.Spec.Type
	}

	// 根据类型选择不同的处理逻辑
	switch endpointType {
	case topohubv1beta1.EndpointTypeSSH:
		return r.handleSSHEndpoint(ctx, hostEndpoint, logger)
	case topohubv1beta1.EndpointTypeRedfish:
		return r.handleRedfishEndpoint(ctx, hostEndpoint, logger)
	default:
		logger.Warnf("Unknown endpoint type: %s, treating as redfish", endpointType)
		return r.handleRedfishEndpoint(ctx, hostEndpoint, logger)
	}
}

// 处理 Redfish 类型的 HostEndpoint
// 根据 HostEndpoint ，同步更新对应的 RedfishStatus
func (r *HostEndpointReconciler) handleRedfishEndpoint(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// Try to get existing RedfishStatus
	existing := &topohubv1beta1.RedfishStatus{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, existing)
	if err == nil {
		// RedfishStatus exists, check if spec changed
		if specEqual(existing.Status.Basic, hostEndpoint.Spec) {
			logger.Debugf("RedfishStatus %s exists with same spec, no update needed", name)
			return nil
		}

		// Spec changed, update the object
		logger.Infof("Updating RedfishStatus %s due to spec change", name)

		// Create a copy of the existing object to avoid modifying the cache
		updated := existing.DeepCopy()
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		updated.Status.Basic = topohubv1beta1.BasicInfo{
			Type:   topohubv1beta1.HostTypeEndpoint,
			IpAddr: hostEndpoint.Spec.IPAddr,
			Https:  true,
			Port:   443,
		}
		if hostEndpoint.Spec.SecretName != nil {
			updated.Status.Basic.SecretName = *hostEndpoint.Spec.SecretName
		}
		if hostEndpoint.Spec.SecretNamespace != nil {
			updated.Status.Basic.SecretNamespace = *hostEndpoint.Spec.SecretNamespace
		}
		if hostEndpoint.Spec.HTTPS != nil {
			updated.Status.Basic.Https = *hostEndpoint.Spec.HTTPS
		}
		if hostEndpoint.Spec.Port != nil {
			updated.Status.Basic.Port = *hostEndpoint.Spec.Port
		}

		if err := r.client.Update(ctx, updated); err != nil {
			if errors.IsConflict(err) {
				logger.Debugf("Conflict updating RedfishStatus %s, will retry", name)
				return err
			}
			logger.Errorf("Failed to update RedfishStatus %s: %v", name, err)
			return err
		}
		logger.Infof("Successfully updated RedfishStatus %s", name)
		logger.Debugf("Updated RedfishStatus details - IP: %s, Secret: %s/%s, Port: %d",
			updated.Status.Basic.IpAddr,
			updated.Status.Basic.SecretNamespace,
			updated.Status.Basic.SecretName,
			updated.Status.Basic.Port)
		return nil
	}

	if !errors.IsNotFound(err) {
		logger.Errorf("Failed to get RedfishStatus %s: %v", name, err)
		return err
	}

	// RedfishStatus doesn't exist, create new one
	redfishStatus := &topohubv1beta1.RedfishStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelIPAddr:     hostEndpoint.Spec.IPAddr,
				topohubv1beta1.LabelClientMode: topohubv1beta1.HostTypeEndpoint,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         topohubv1beta1.APIVersion,
					Kind:               topohubv1beta1.KindHostEndpoint,
					Name:               hostEndpoint.Name,
					UID:                hostEndpoint.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
	}

	logger.Debugf("Creating new RedfishStatus %s", name)
	if err := r.client.Create(ctx, redfishStatus); err != nil {
		logger.Errorf("Failed to create RedfishStatus %s: %v", name, err)
		return err
	}
	logger.Infof("Successfully created RedfishStatus %s", name)
	return nil
}

// specEqual checks if the RedfishStatus basic info matches the HostEndpoint spec
func specEqual(basic topohubv1beta1.BasicInfo, spec topohubv1beta1.HostEndpointSpec) bool {
	// 检查 IP 地址是否匹配
	if basic.IpAddr != spec.IPAddr {
		return false
	}

	clusterName := ""
	if spec.ClusterName != nil {
		clusterName = *spec.ClusterName
	}
	if clusterName != basic.ClusterName {
		return false
	}

	if !(spec.SecretName != nil && basic.SecretName == *spec.SecretName) {
		return false
	}

	if !(spec.SecretNamespace != nil && basic.SecretNamespace == *spec.SecretNamespace) {
		return false
	}

	if !(spec.HTTPS != nil && basic.Https == *spec.HTTPS) {
		return false
	}
	if !(spec.Port != nil && basic.Port == *spec.Port) {
		return false
	}

	if spec.Type != nil {
		if basic.Type != *spec.Type {
			return false
		}
	} else if basic.Type != topohubv1beta1.EndpointTypeRedfish {
		return false
	}

	return true
}

// Handle SSH type HostEndpoint
func (r *HostEndpointReconciler) handleSSHEndpoint(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing SSH HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// Try to get existing SSHStatus
	existing := &topohubv1beta1.SSHStatus{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, existing)
	if err == nil {
		// SSHStatus exists, check if spec changed
		if specEqualSSH(existing.Status.Basic, hostEndpoint.Spec) {
			logger.Debugf("SSHStatus %s exists with same spec, no update needed", name)
			return nil
		}

		// Spec changed, update the object
		logger.Infof("Updating SSHStatus %s due to spec change", name)

		// Create a copy of the existing object to avoid modifying the cache
		updated := existing.DeepCopy()
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		updated.Status.Basic = topohubv1beta1.SSHBasicInfo{
			Type:   topohubv1beta1.HostTypeSSH,
			IpAddr: hostEndpoint.Spec.IPAddr,
			Port:   *hostEndpoint.Spec.Port,
		}

		// Ensure Info field is initialized
		if updated.Status.Info == nil {
			updated.Status.Info = map[string]string{}
		}

		if hostEndpoint.Spec.SecretName != nil {
			updated.Status.Basic.SecretName = *hostEndpoint.Spec.SecretName
		}
		if hostEndpoint.Spec.SecretNamespace != nil {
			updated.Status.Basic.SecretNamespace = *hostEndpoint.Spec.SecretNamespace
		}
		if hostEndpoint.Spec.Port != nil {
			updated.Status.Basic.Port = *hostEndpoint.Spec.Port
		}
		if hostEndpoint.Spec.ClusterName != nil {
			updated.Status.Basic.ClusterName = *hostEndpoint.Spec.ClusterName
		}

		// Output detailed information before update
		logger.Debugf("Updating SSHStatus with details - IP: %s, Secret: %s/%s, Port: %d, ClusterName: %s",
			updated.Status.Basic.IpAddr,
			updated.Status.Basic.SecretNamespace,
			updated.Status.Basic.SecretName,
			updated.Status.Basic.Port,
			updated.Status.Basic.ClusterName)

		if err := r.client.Status().Update(ctx, updated); err != nil {
			if errors.IsConflict(err) {
				logger.Debugf("Conflict updating SSHStatus %s, will retry", name)
				return err
			}
			logger.Errorf("Failed to update SSHStatus %s: %v", name, err)
			return err
		}
		logger.Infof("Successfully updated SSHStatus %s", name)
		logger.Debugf("Updated SSHStatus details - IP: %s, Secret: %s/%s, Port: %d",
			updated.Status.Basic.IpAddr,
			updated.Status.Basic.SecretNamespace,
			updated.Status.Basic.SecretName,
			updated.Status.Basic.Port)
		return nil
	}

	if !errors.IsNotFound(err) {
		logger.Errorf("Failed to get SSHStatus %s: %v", name, err)
		return err
	}

	// SSHStatus doesn't exist, create a new one
	sshStatus := &topohubv1beta1.SSHStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelIPAddr:     hostEndpoint.Spec.IPAddr,
				topohubv1beta1.LabelClientMode: topohubv1beta1.HostTypeSSH,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         topohubv1beta1.APIVersion,
					Kind:               topohubv1beta1.KindHostEndpoint,
					Name:               hostEndpoint.Name,
					UID:                hostEndpoint.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
	}

	logger.Debugf("Creating new SSHStatus %s", name)
	if err := r.client.Create(ctx, sshStatus); err != nil {
		logger.Errorf("Failed to create SSHStatus %s: %v", name, err)
		return err
	}
	return nil
}

// Check if the SSH status basic info matches the host endpoint spec
func specEqualSSH(basic topohubv1beta1.SSHBasicInfo, spec topohubv1beta1.HostEndpointSpec) bool {
	if basic.IpAddr != spec.IPAddr {
		return false
	}

	// Check port
	if spec.Port != nil && basic.Port != *spec.Port {
		return false
	}

	// Check Secret
	if spec.SecretName != nil && basic.SecretName != *spec.SecretName {
		return false
	}

	// Check Secret namespace
	if spec.SecretNamespace != nil && basic.SecretNamespace != *spec.SecretNamespace {
		return false
	}

	// Check cluster name
	clusterName := ""
	if spec.ClusterName != nil {
		clusterName = *spec.ClusterName
	}
	return basic.ClusterName == clusterName
}

// SetupWithManager sets up the controller with the Manager
func (r *HostEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.HostEndpoint{}).
		Complete(r)
}
