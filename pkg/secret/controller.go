package secret

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/redfishstatus"
)

type SecretReconciler struct {
	client                  client.Client
	config                  *config.AgentConfig
	redfishStatusController redfishstatus.RedfishStatusController
	log                     *zap.SugaredLogger
}

// NewHostEndpointReconciler creates a new HostEndpoint reconciler
func NewSecretReconciler(mgr ctrl.Manager, config *config.AgentConfig, redfishStatusController redfishstatus.RedfishStatusController) (*SecretReconciler, error) {
	return &SecretReconciler{
		client:                  mgr.GetClient(),
		config:                  config,
		redfishStatusController: redfishStatusController,
		log:                     log.Logger.Named("secretReconciler"),
	}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create predicate to filter secrets by label
	redfishSecretPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		labels := obj.GetLabels()
		if labels == nil {
			return false
		}
		// Check if the secret has the secret-credential label (any value)
		if _, exists := labels[topohubv1beta1.LabelSecretCredential]; exists {
			return true
		}
		return false
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(redfishSecretPredicate).
		Complete(r)
}

// Reconcile handles the reconciliation of HostEndpoint objects
func (r *SecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With("secret", req.Name)

	logger.Debugf("Reconciling Secret %s", req.Name)

	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("Secret not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get Secret")
		return reconcile.Result{}, err
	}

	username, ok := secret.Data["username"]
	if !ok {
		logger.Debugf("Secret %s/%s does not contain username field, ignoring", secret.Namespace, secret.Name)
		return reconcile.Result{}, nil
	}
	password, ok := secret.Data["password"]
	if !ok {
		logger.Debugf("Secret %s/%s does not contain password field, ignoring", secret.Namespace, secret.Name)
		return reconcile.Result{}, nil
	}

	logger.Debugf("Retrieved new secret data for %s/%s", secret.Namespace, secret.Name)

	// 获取所有 HostEndpoint 资源
	hostEndpoints := &topohubv1beta1.HostEndpointList{}
	if err := r.client.List(ctx, hostEndpoints); err != nil {
		logger.Errorf("Failed to list HostEndpoints: %v", err)
		return reconcile.Result{}, err
	}

	// 筛选出使用变更 Secret 的 HostEndpoint
	affectedCount := 0
	for _, hostEndpoint := range hostEndpoints.Items {
		// 检查 HostEndpoint 是否使用了这个 Secret
		if hostEndpoint.Spec.SecretName != nil && hostEndpoint.Spec.SecretNamespace != nil &&
			*hostEndpoint.Spec.SecretName == secret.Name && *hostEndpoint.Spec.SecretNamespace == secret.Namespace {

			// 调用 redfishStatusController.UpdateSecret 更新连接缓存
			logger.Infof("Updating connection cache for HostEndpoint %s using Secret %s/%s",
				hostEndpoint.Name, secret.Namespace, secret.Name)

			r.redfishStatusController.UpdateSecret(
				secret.Name,
				secret.Namespace,
				string(username),
				string(password),
			)
			affectedCount++
		}
	}

	logger.Infof("Updated connection cache for %d HostEndpoints using Secret %s/%s",
		affectedCount, secret.Namespace, secret.Name)

	return reconcile.Result{}, nil
}
