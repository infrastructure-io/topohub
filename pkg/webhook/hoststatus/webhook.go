package hoststatus

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// +kubebuilder:webhook:path=/mutate-topohub-infrastructure-io-v1beta1-hoststatus,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=hoststatuses,verbs=create;update,versions=v1beta1,name=mhoststatus.kb.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-topohub-infrastructure-io-v1beta1-hoststatus,mutating=false,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=hoststatuses,verbs=create;update,versions=v1beta1,name=vhoststatus.kb.io,admissionReviewVersions=v1

// HostStatusWebhook validates HostStatus resources
type HostStatusWebhook struct {
	Client client.Client
}

func (w *HostStatusWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&topohubv1beta1.HostStatus{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default implements webhook.Defaulter
func (w *HostStatusWebhook) Default(ctx context.Context, obj runtime.Object) error {
	hoststatus, ok := obj.(*topohubv1beta1.HostStatus)
	if !ok {
		err := fmt.Errorf("object is not a HostStatus")
		log.Logger.Error(err.Error())
		return err
	}

	log.Logger.Debugf("Processing Default webhook for HostStatus %s", hoststatus.Name)

	if hoststatus.ObjectMeta.Labels == nil {
		hoststatus.ObjectMeta.Labels = make(map[string]string)
	}
	// cluster name
	hoststatus.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] = hoststatus.Status.Basic.ClusterName
	// ip
	IpAddr := strings.Split(hoststatus.Status.Basic.IpAddr, "/")[0]
	hoststatus.ObjectMeta.Labels[topohubv1beta1.LabelIPAddr] = IpAddr
	// mode
	if hoststatus.Status.Basic.Type == topohubv1beta1.HostTypeDHCP {
		hoststatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeDHCP
	} else {
		hoststatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeEndpoint
	}
	// dhcp
	if hoststatus.Status.Basic.ActiveDhcpClient {
		hoststatus.ObjectMeta.Labels[topohubv1beta1.LabelClientActive] = "true"
	} else {
		hoststatus.ObjectMeta.Labels[topohubv1beta1.LabelClientActive] = "false"
	}

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *HostStatusWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	hoststatus, ok := obj.(*topohubv1beta1.HostStatus)
	if !ok {
		err := fmt.Errorf("object is not a HostStatus")
		log.Logger.Error(err.Error())
		return nil, err
	}

	log.Logger.Debugf("Validating creation of HostStatus %s", hoststatus.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *HostStatusWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	hoststatus, ok := newObj.(*topohubv1beta1.HostStatus)
	if !ok {
		err := fmt.Errorf("object is not a HostStatus")
		log.Logger.Error(err.Error())
		return nil, err
	}

	log.Logger.Debugf("Validating update of HostStatus %s", hoststatus.Name)

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *HostStatusWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
