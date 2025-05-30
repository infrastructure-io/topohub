package redfishstatus

import (
	"context"
	"fmt"
	"strings"

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
	redfishstatus, ok := obj.(*topohubv1beta1.RedfishStatus)
	if !ok {
		err := fmt.Errorf("object is not a RedfishStatus")
		w.log.Error(err.Error())
		return err
	}

	w.log.Debugf("Processing Default webhook for RedfishStatus %s", redfishstatus.Name)

	if redfishstatus.ObjectMeta.Labels == nil {
		redfishstatus.ObjectMeta.Labels = make(map[string]string)
	}

	// cluster name
	w.log.Debugf("Setting ClusterName label for RedfishStatus %s: %s",
		redfishstatus.Name, redfishstatus.Status.Basic.ClusterName)
	redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelClusterName] = redfishstatus.Status.Basic.ClusterName

	// ip
	w.log.Debugf("Processing IpAddr for RedfishStatus %s: %s",
		redfishstatus.Name, redfishstatus.Status.Basic.IpAddr)
	IpAddr := strings.Split(redfishstatus.Status.Basic.IpAddr, "/")[0]
	w.log.Debugf("Setting IpAddr label for RedfishStatus %s: %s",
		redfishstatus.Name, IpAddr)
	redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelIPAddr] = IpAddr

	// mode
	w.log.Debugf("Setting ClientMode label for RedfishStatus %s based on type: %s",
		redfishstatus.Name, redfishstatus.Status.Basic.Type)
	if redfishstatus.Status.Basic.Type == topohubv1beta1.HostTypeDHCP {
		redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeDHCP
	} else {
		redfishstatus.ObjectMeta.Labels[topohubv1beta1.LabelClientMode] = topohubv1beta1.HostTypeEndpoint
	}

	w.log.Debugf("Finished processing webhook for RedfishStatus %s", redfishstatus.Name)

	return nil
}

// ValidateCreate implements webhook.Validator
func (w *RedfishStatusWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	redfishstatus, ok := obj.(*topohubv1beta1.RedfishStatus)
	if !ok {
		err := fmt.Errorf("object is not a RedfishStatus")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Debugf("Validating creation of RedfishStatus %s", redfishstatus.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *RedfishStatusWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	redfishstatus, ok := newObj.(*topohubv1beta1.RedfishStatus)
	if !ok {
		err := fmt.Errorf("object is not a RedfishStatus")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Debugf("Validating update of RedfishStatus %s", redfishstatus.Name)

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *RedfishStatusWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
