package hostoperation

import (
	"context"
	"fmt"
	"slices"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// global variables define valid operation types and actions
var (
	// Redfish valid actions list
	redfishValidActions = []string{
		topohubv1beta1.RedfishCmdOn,
		topohubv1beta1.RedfishCmdForceOn,
		topohubv1beta1.RedfishCmdForceOff,
		topohubv1beta1.RedfishCmdGracefulShutdown,
		topohubv1beta1.RedfishCmdForceRestart,
		topohubv1beta1.RedfishCmdGracefulRestart,
		topohubv1beta1.RedfishCmdPxeReboot,
	}

	// SSH valid actions list
	sshValidActions = []string{
		topohubv1beta1.SSHCmdShutdown,
		topohubv1beta1.SSHCmdRestart,
	}
)



type HostOperationWebhook struct {
	Client client.Client
	log    *zap.SugaredLogger
}

func (h *HostOperationWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	h.Client = mgr.GetClient()
	h.log = log.Logger.Named("hostoperationWebhook")
	log.Logger.Info("Setting up HostOperation webhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&topohubv1beta1.HostOperation{}).
		WithValidator(h).
		WithDefaulter(h).
		Complete()
}

func (h *HostOperationWebhook) Default(ctx context.Context, obj runtime.Object) error {
	hostOp, ok := obj.(*topohubv1beta1.HostOperation)
	if !ok {
		err := fmt.Errorf("expected a HostOperation but got a %T", obj)
		h.log.Error(err.Error())
		return err
	}

	h.log.Debugf("Processing Default webhook for HostOperation %s", hostOp.Name)

	h.log.Debugf("Successfully processed Default webhook for HostOperation %s", hostOp.Name)
	return nil
}

// +kubebuilder:webhook:path=/validate-topohub-infrastructure-io-v1beta1-hostoperation,mutating=false,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=hostoperations,verbs=create;update,versions=v1beta1,name=vhostoperation.kb.io,admissionReviewVersions=v1

func (h *HostOperationWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	hostOp, ok := obj.(*topohubv1beta1.HostOperation)
	if !ok {
		err := fmt.Errorf("expected a HostOperation but got a %T", obj)
		h.log.Error(err.Error())
		return nil, err
	}

	h.log.Debugf("Processing ValidateCreate webhook for HostOperation %s", hostOp.Name)

	var err error
	switch hostOp.Spec.HostType {
	case "Redfish":
		if !slices.Contains(redfishValidActions, hostOp.Spec.Action) {
			err = fmt.Errorf("invalid action %s for Redfish operation type", hostOp.Spec.Action)
		}
	case "SSH":
		if !slices.Contains(sshValidActions, hostOp.Spec.Action) {
			err = fmt.Errorf("invalid action %s for SSH operation type", hostOp.Spec.Action)
		}
	default:
		err = fmt.Errorf("invalid type %s, must be either 'Redfish' or 'SSH'", hostOp.Spec.HostType)
	}

	if err != nil {
		h.log.Error(err.Error())
		return nil, err
	}

	h.log.Debugf("Successfully validated HostOperation %s creation", hostOp.Name)
	return nil, nil
}

func (h *HostOperationWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	hostOp, ok := oldObj.(*topohubv1beta1.HostOperation)
	if !ok {
		err := fmt.Errorf("expected a HostOperation but got a %T", oldObj)
		h.log.Error(err.Error())
		return nil, err
	}
	h.log.Debugf("Rejecting update of HostOperation %s: updates are not allowed", hostOp.Name)
	return nil, fmt.Errorf("updates to HostOperation resources are not allowed")
}

func (h *HostOperationWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	hostOp, ok := obj.(*topohubv1beta1.HostOperation)
	if !ok {
		err := fmt.Errorf("expected a HostOperation but got a %T", obj)
		h.log.Error(err.Error())
		return nil, err
	}

	h.log.Debugf("Processing ValidateDelete webhook for HostOperation %s", hostOp.Name)
	return nil, nil
}
