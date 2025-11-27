package hostendpoint

import (
	"context"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/hostendpoint/handler"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/redfishstatus"
	"github.com/infrastructure-io/topohub/pkg/sshstatus"
)

// HostEndpointReconciler reconciles a HostEndpoint object
type HostEndpointReconciler struct {
	client            client.Client
	handlers          map[string]handler.HostEndpointHandler
	log               *zap.SugaredLogger
	redfishStatusCtrl redfishstatus.RedfishStatusController
	sshStatusCtrl     sshstatus.SSHStatusController
}

// NewHostEndpointReconciler creates a new HostEndpoint reconciler
func NewHostEndpointReconciler(mgr ctrl.Manager, redfishStatusCtrl redfishstatus.RedfishStatusController, sshStatusCtrl sshstatus.SSHStatusController) *HostEndpointReconciler {
	return &HostEndpointReconciler{
		client:            mgr.GetClient(),
		handlers:          handler.GetHandlerRegistry(mgr.GetClient(), mgr.GetCache()),
		log:               log.Logger.Named("hostendpointReconcile"),
		redfishStatusCtrl: redfishStatusCtrl,
		sshStatusCtrl:     sshStatusCtrl,
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
		return reconcile.Result{}, nil
	}

	// determine endpoint type
	isSSH := false
	if t := hostEndpoint.Spec.Type; t != nil && *t == topohubv1beta1.EndpointTypeSSH {
		isSSH = true
	}

	var handler handler.HostEndpointHandler
	if isSSH {
		handler = r.handlers[topohubv1beta1.EndpointTypeSSH]
	} else { // Default type is redfish
		handler = r.handlers[topohubv1beta1.EndpointTypeRedfish]
	}
	if handler == nil {
		logger.Errorf("Failed to get HostEndpoint handler, invalid endpoint type '%s'",
			hostEndpoint.Spec.Type)
		return reconcile.Result{}, nil
	}
	logger.Debugf("Processing HostEndpoint (IP: %s)", hostEndpoint.Spec.IPAddr)
	if err := handler.RefreshSession(ctx, &hostEndpoint, nil, logger); err != nil {
		logger.Errorf("Failed to refresh session, err: %v", err)
	}

	created, err := handler.CreateStatusIfNotExist(ctx, &hostEndpoint, logger)
	if err != nil {
		logger.Errorf("Failed to create RedfishStatus, err: %v", err)
		return reconcile.Result{}, nil
	}

	if created {
		logger.Info("RedfishStatus newly created, no need to update info")
		return reconcile.Result{}, nil
	}

	logger.Info("Updating Status info")
	r.updateStatus(ctx, hostEndpoint.Name, isSSH, logger)

	return reconcile.Result{}, nil
}

func (r *HostEndpointReconciler) updateStatus(ctx context.Context, name string, isSSH bool, logger *zap.SugaredLogger) {
	// select update logic based on endpoint type
	if isSSH {
		// SSH type - update SSHStatus
		logger.Info("Triggering SSHStatus info update")

		// get SSHStatus
		var sshStatus topohubv1beta1.SSHStatus
		if err := r.client.Get(ctx, client.ObjectKey{Name: name}, &sshStatus); err != nil {
			logger.Errorf("Failed to get SSHStatus for update: %v", err)
			return
		}

		// call update method
		if err := r.sshStatusCtrl.UpdateSSHStatusInfo(&sshStatus); err != nil {
			logger.Errorf("Failed to update SSHStatus info: %v", err)
		} else {
			logger.Info("Successfully triggered SSHStatus info update")
		}
	} else {
		// Redfish type - update RedfishStatus
		logger.Info("Triggering RedfishStatus info update")

		// get RedfishStatus
		var redfishStatus topohubv1beta1.RedfishStatus
		if err := r.client.Get(ctx, client.ObjectKey{Name: name}, &redfishStatus); err != nil {
			logger.Errorf("Failed to get RedfishStatus for update: %v", err)
			return
		}

		// call update method
		if err := r.redfishStatusCtrl.UpdateRedfishStatusInfo(&redfishStatus); err != nil {
			logger.Errorf("Failed to update RedfishStatus info: %v", err)
		} else {
			logger.Info("Successfully triggered RedfishStatus info update")
		}
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *HostEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.HostEndpoint{}).
		Complete(r)
}
