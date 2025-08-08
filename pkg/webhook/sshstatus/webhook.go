package sshstatus

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// +kubebuilder:webhook:path=/mutate-topohub-infrastructure-io-v1beta1-sshstatus,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=sshstatuses,verbs=create;update,versions=v1beta1,name=msshstatus.kb.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-topohub-infrastructure-io-v1beta1-sshstatus,mutating=false,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=sshstatuses,verbs=create;update,versions=v1beta1,name=vsshstatus.kb.io,admissionReviewVersions=v1

// SSHStatusWebhook validates SSHStatus resources
type SSHStatusWebhook struct {
	Client client.Client
	log    *zap.SugaredLogger
}

// SetupWebhookWithManager sets up the webhook with the Manager
func (w *SSHStatusWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
	w.log = log.Logger.Named("sshstatusWebhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&topohubv1beta1.SSHStatus{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default implements webhook.Defaulter
func (w *SSHStatusWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator
func (w *SSHStatusWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *SSHStatusWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *SSHStatusWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
