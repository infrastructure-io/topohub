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

// Reconcile handles the reconciliation of HostEndpoint objects
func (r *HostEndpointReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With("hostendpoint", req.Name)

	// get the HostEndpoint
	hostEndpoint := &topohubv1beta1.HostEndpoint{}
	if err := r.client.Get(ctx, req.NamespacedName, hostEndpoint); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("HostEndpoint not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get HostEndpoint")
		return reconcile.Result{}, err
	}

	// handle the HostEndpoint
	if err := r.handleHostEndpoint(ctx, hostEndpoint, logger); err != nil {
		logger.Error(err, "Failed to handle HostEndpoint")
		return reconcile.Result{
			RequeueAfter: time.Second * 2,
		}, err
	}

	return reconcile.Result{}, nil
}

// HandleHostEndpoint handles the HostEndpoint object
func (r *HostEndpointReconciler) handleHostEndpoint(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// get the type of HostEndpoint, default is redfish
	endpointType := topohubv1beta1.EndpointTypeRedfish
	if hostEndpoint.Spec.Type != nil {
		endpointType = *hostEndpoint.Spec.Type
	}

	// according to the type of HostEndpoint, call different handler
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

// handle Redfish endpoint
func (r *HostEndpointReconciler) handleRedfishEndpoint(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// try to get existing RedfishStatus
	existing := &topohubv1beta1.RedfishStatus{}

	// return nil if RedfishStatus already exists
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, existing)
	if err == nil {
		logger.Infof("RedfishStatus %s already exists, no need to create", name)
		return nil
	}

	// if error is not not found, return error
	if !errors.IsNotFound(err) {
		logger.Errorf("Failed to get RedfishStatus %s: %v", name, err)
		return err
	}

	// RedfishStatus doesn't exist, create new one
	redfishStatus := &topohubv1beta1.RedfishStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelClientMode: topohubv1beta1.Redfish,
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

// Handle SSH type HostEndpoint
func (r *HostEndpointReconciler) handleSSHEndpoint(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing SSH HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// Try to get existing SSHStatus
	existing := &topohubv1beta1.SSHStatus{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, existing)
	if err == nil {
		logger.Infof("SSHStatus %s already exists, no need to create", name)
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
				topohubv1beta1.LabelClientMode: topohubv1beta1.SSH,
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

// SetupWithManager sets up the controller with the Manager
func (r *HostEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.HostEndpoint{}).
		Complete(r)
}
