package hostendpoint

import (
	"context"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/hostendpoint/handler"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// HostEndpointReconciler reconciles a HostEndpoint object
type HostEndpointReconciler struct {
	client   client.Client
	handlers map[string]handler.HostEndpointHandler
	log      *zap.SugaredLogger
}

// NewHostEndpointReconciler creates a new HostEndpoint reconciler
func NewHostEndpointReconciler(mgr ctrl.Manager) *HostEndpointReconciler {
	return &HostEndpointReconciler{
		client:   mgr.GetClient(),
		handlers: handler.RegisterHostEndpointHandlers(mgr.GetClient(), mgr.GetCache()),
		log:      log.Logger.Named("hostendpointReconcile"),
	}
}

// Reconcile handles the reconciliation of HostEndpoint objects
func (r *HostEndpointReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With("hostendpoint", req.Name)

	// get the HostEndpoint
	var hostEndpoint topohubv1beta1.HostEndpoint
	if err := r.client.Get(ctx, req.NamespacedName, &hostEndpoint); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("HostEndpoint not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get HostEndpoint")
		return reconcile.Result{RequeueAfter: time.Second * 1}, nil
	}

	var handler handler.HostEndpointHandler
	if t := hostEndpoint.Spec.Type; t != nil && *t == topohubv1beta1.EndpointTypeSSH {
		handler = r.handlers[topohubv1beta1.EndpointTypeSSH]
	} else { // Default type is redfish
		handler = r.handlers[topohubv1beta1.EndpointTypeRedfish]
	}
	if handler == nil {
		logger.Errorf("Failed to get HostEndpoint handler, invalid endpoint type '%s'",
			hostEndpoint.Spec.Type)
		return reconcile.Result{}, nil
	}
	logger.Debugf("Processing SSH HostEndpoint (IP: %s)", hostEndpoint.Spec.IPAddr)
	if err := handler.RefreshSession(ctx, &hostEndpoint, nil, logger); err != nil {
		logger.Errorf("Failed to refresh session, err: %v", err)
	}
	if _, err := handler.CreateStatusIfNotExist(ctx, &hostEndpoint, logger); err != nil {
		logger.Errorf("Failed to create RedfishStatus, err: %v", err)
		if !apierrors.IsConflict(err) {
			return reconcile.Result{RequeueAfter: time.Second * 2}, nil
		}
	}
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *HostEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.HostEndpoint{}).
		Complete(r)
}
