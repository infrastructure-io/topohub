package sshstatus

import (
	"context"
	"fmt"

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
	sshstatus, ok := obj.(*topohubv1beta1.SSHStatus)
	if !ok {
		err := fmt.Errorf("object is not a SSHStatus")
		w.log.Error(err.Error())
		return err
	}

	w.log.Debugf("Processing Default webhook for SSHStatus %s", sshstatus.Name)

	if sshstatus.ObjectMeta.Labels == nil {
		sshstatus.ObjectMeta.Labels = make(map[string]string)
	}

	// Add IP address label if not present
	if sshstatus.Status.Basic.IpAddr != "" && sshstatus.ObjectMeta.Labels[topohubv1beta1.LabelIPAddr] == "" {
		sshstatus.ObjectMeta.Labels[topohubv1beta1.LabelIPAddr] = sshstatus.Status.Basic.IpAddr
	}

	// Add client mode label if not present
	if sshstatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] == "" {
		sshstatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeSSH
	}

	// Add cluster name label if not present
	if sshstatus.Status.Basic.ClusterName != "" && sshstatus.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] == "" {
		sshstatus.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] = sshstatus.Status.Basic.ClusterName
	}

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *SSHStatusWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sshstatus, ok := obj.(*topohubv1beta1.SSHStatus)
	if !ok {
		err := fmt.Errorf("object is not a SSHStatus")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Debugf("Validating creation of SSHStatus %s", sshstatus.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *SSHStatusWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	sshstatus, ok := newObj.(*topohubv1beta1.SSHStatus)
	if !ok {
		err := fmt.Errorf("object is not a SSHStatus")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Debugf("Validating update of SSHStatus %s", sshstatus.Name)

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *SSHStatusWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
