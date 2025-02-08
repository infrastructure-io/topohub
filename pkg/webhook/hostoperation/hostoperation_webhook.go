package hostoperation

import (
	"context"
	"fmt"
	"go.uber.org/zap"

	//"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// +kubebuilder:webhook:path=/mutate-topohub-infrastructure-io-v1beta1-hostoperation,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=hostoperations,verbs=create;update,versions=v1beta1,name=mhostoperation.kb.io,admissionReviewVersions=v1

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

	// 验证 hostStatusName 对应的 HostStatus 是否存在且健康
	var hostStatus topohubv1beta1.HostStatus
	if err := h.Client.Get(ctx, client.ObjectKey{Name: hostOp.Spec.HostStatusName}, &hostStatus); err != nil {
		err = fmt.Errorf("hostStatus %s not found: %v", hostOp.Spec.HostStatusName, err)
		h.log.Error(err.Error())
		return nil, err
	}

	if !hostStatus.Status.Healthy {
		err := fmt.Errorf("hostStatus %s is not healthy, so it is not allowed to create hostOperation %s", hostOp.Spec.HostStatusName, hostOp.Name)
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
