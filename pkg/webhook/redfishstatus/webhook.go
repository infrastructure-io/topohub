package redfishstatus

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

// +kubebuilder:webhook:path=/mutate-topohub-infrastructure-io-v1beta1-redfishstatus,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=redfishstatuses,verbs=create;update,versions=v1beta1,name=mredfishstatus.kb.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-topohub-infrastructure-io-v1beta1-redfishstatus,mutating=false,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=redfishstatuses,verbs=create;update,versions=v1beta1,name=vredfishstatus.kb.io,admissionReviewVersions=v1

// RedfishStatusWebhook validates RedfishStatus resources
type RedfishStatusWebhook struct {
	Client client.Client
	log    *zap.SugaredLogger
}

// SetupWebhookWithManager sets up the webhook with the manager
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	webhook := &RedfishStatusWebhook{
		Client: mgr.GetClient(),
		log:    log.Logger.Named("redfishstatusWebhook"),
	}
	return ctrl.NewWebhookManagedBy(mgr).
		For(&topohubv1beta1.RedfishStatus{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

// Default implements webhook.Defaulter
func (w *RedfishStatusWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator
func (w *RedfishStatusWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *RedfishStatusWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *RedfishStatusWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
