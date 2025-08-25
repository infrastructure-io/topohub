package secret

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/clients/kube"
	"github.com/infrastructure-io/topohub/pkg/hostendpoint/handler"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

type SecretReconciler struct {
	client   client.Client
	log      *zap.SugaredLogger
	handlers map[string]handler.HostEndpointHandler // handlers for different endpoint types
}

// NewSecretReconciler creates a new Secret reconciler
func NewSecretReconciler(mgr ctrl.Manager) (*SecretReconciler, error) {
	// Get global handler registry
	handlers := handler.GetHandlerRegistry(mgr.GetClient(), mgr.GetCache())

	return &SecretReconciler{
		client:   mgr.GetClient(),
		log:      log.Logger.Named("secretReconciler"),
		handlers: handlers,
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

// Reconcile handles the reconciliation of Secret objects
func (r *SecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With("secret", req.Name)

	logger.Debugf("Reconciling Secret %s", req.Name)

	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Debugf("Secret not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get Secret")
		return reconcile.Result{}, nil
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

	// get all HostEndpoint resources
	hostEndpoints := &topohubv1beta1.HostEndpointList{}
	if err := r.client.List(ctx, hostEndpoints); err != nil {
		logger.Errorf("failed to list HostEndpoints: %v", err)
		return reconcile.Result{}, nil
	}

	// filter out HostEndpoint that uses this secret and process them
	for i := range hostEndpoints.Items {
		hostEndpoint := hostEndpoints.Items[i]

		// check if HostEndpoint uses this secret
		if hostEndpoint.Spec.SecretName != nil && hostEndpoint.Spec.SecretNamespace != nil &&
			*hostEndpoint.Spec.SecretName == secret.Name && *hostEndpoint.Spec.SecretNamespace == secret.Namespace {

			// refresh session pool cache based on HostEndpoint type
			logger.Infof("refreshing session for HostEndpoint %s (type: %s) using Secret %s/%s",
				hostEndpoint.Name, hostEndpoint.Spec.Type, secret.Namespace, secret.Name)

			// Create authentication secret from the data
			auth := &kube.AuthenticationSecret{
				Username: string(username),
				Password: string(password),
			}

			// Get the appropriate handler for this endpoint type
			if hostEndpoint.Spec.Type == nil {
				logger.Errorf("HostEndpoint %s has no type specified", hostEndpoint.Name)
				continue
			}

			h := r.handlers[*hostEndpoint.Spec.Type]
			if h == nil {
				logger.Errorf("no handler found for HostEndpoint type %s", *hostEndpoint.Spec.Type)
				continue
			}

			// Use the handler to refresh the session (now in serial)
			if err := h.RefreshSession(context.Background(), &hostEndpoint, auth, logger); err != nil {
				logger.Errorf("failed to refresh session for HostEndpoint %s: %v", hostEndpoint.Name, err)
			}
		}
	}

	logger.Infof("updated connection cache for Secret %s/%s", secret.Namespace, secret.Name)

	return reconcile.Result{}, nil
}
