package bindingip

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"net"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/tools"
)

// +kubebuilder:webhook:path=/validate-topohub-infrastructure-io-v1beta1-bindingip,mutating=true,failurePolicy=fail,sideEffects=None,groups=topohub.infrastructure.io,resources=bindingips,verbs=create;update,versions=v1beta1,name=vbindingip.kb.io,admissionReviewVersions=v1

// BindingIPWebhook validates BindingIP resources
type BindingIPWebhook struct {
	Client client.Client
	config *config.AgentConfig
	log    *zap.SugaredLogger
}

func (w *BindingIPWebhook) SetupWebhookWithManager(mgr ctrl.Manager, config config.AgentConfig) error {
	w.Client = mgr.GetClient()
	w.config = &config
	w.log = log.Logger.Named("bindingipWebhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&topohubv1beta1.BindingIp{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default implements webhook.Defaulter
func (w *BindingIPWebhook) Default(ctx context.Context, obj runtime.Object) error {
	bindingIP, ok := obj.(*topohubv1beta1.BindingIp)
	if !ok {
		return fmt.Errorf("object is not a BindingIP")
	}

	if bindingIP.ObjectMeta.Labels == nil {
		bindingIP.ObjectMeta.Labels = make(map[string]string)
	}
	bindingIP.ObjectMeta.Labels[topohubv1beta1.LabelSubnetName] = bindingIP.Spec.Subnet

	w.log.Debugf("Setting initial values for nil fields in BindingIP %s", bindingIP.Name)
	return nil
}

// ValidateCreate implements webhook.Validator
func (w *BindingIPWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	bindingIP, ok := obj.(*topohubv1beta1.BindingIp)
	if !ok {
		err := fmt.Errorf("object is not a BindingIP")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Debugf("Validating creation of BindingIP %s", bindingIP.Name)

	if err := w.validateBindingIP(ctx, bindingIP); err != nil {
		w.log.Errorf("Failed to validate BindingIP %s: %v", bindingIP.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (w *BindingIPWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	bindingIP, ok := newObj.(*topohubv1beta1.BindingIp)
	if !ok {
		err := fmt.Errorf("object is not a BindingIP")
		w.log.Error(err.Error())
		return nil, err
	}

	w.log.Debugf("Validating update of BindingIP %s", bindingIP.Name)

	if err := w.validateBindingIP(ctx, bindingIP); err != nil {
		w.log.Errorf("Failed to validate BindingIP %s: %v", bindingIP.Name, err)
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (w *BindingIPWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateBindingIP validates the BindingIP resource
func (w *BindingIPWebhook) validateBindingIP(ctx context.Context, bindingIP *topohubv1beta1.BindingIp) error {
	// 1. 校验 MAC 地址是否为单播地址
	if !tools.IsValidUnicastMAC(bindingIP.Spec.MacAddr) {
		return fmt.Errorf("invalid unicast MAC address: %s", bindingIP.Spec.MacAddr)
	}

	// 2. 校验对应的 subnet 是否存在
	subnet := &topohubv1beta1.Subnet{}
	if err := w.Client.Get(ctx, client.ObjectKey{Name: bindingIP.Spec.Subnet}, subnet); err != nil {
		return fmt.Errorf("failed to get subnet %s: %v", bindingIP.Spec.Subnet, err)
	}

	// 3. 校验 IP 地址是否在子网范围内
	ip := net.ParseIP(bindingIP.Spec.IpAddr)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", bindingIP.Spec.IpAddr)
	}

	if !tools.IsIPInRange(ip, subnet.Spec.IPv4Subnet.IPRange) {
		return fmt.Errorf("IP address %s is not in subnet %s IP range: %s", 
			bindingIP.Spec.IpAddr, 
			bindingIP.Spec.Subnet, 
			subnet.Spec.IPv4Subnet.IPRange)
	}

	// 4. 校验 IP 地址是否已被其他 BindingIP 使用
	bindingIPList := &topohubv1beta1.BindingIpList{}
	if err := w.Client.List(ctx, bindingIPList); err != nil {
		return fmt.Errorf("failed to list BindingIPs: %v", err)
	}

	for _, existingBindingIP := range bindingIPList.Items {
		// 跳过自身
		if existingBindingIP.Name == bindingIP.Name {
			continue
		}
		if existingBindingIP.Spec.IpAddr == bindingIP.Spec.IpAddr {
			return fmt.Errorf("IP address %s is already used by BindingIP %s", 
				bindingIP.Spec.IpAddr, 
				existingBindingIP.Name)
		}
	}

	return nil
}
