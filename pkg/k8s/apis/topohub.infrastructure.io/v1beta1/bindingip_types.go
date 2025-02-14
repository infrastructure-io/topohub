package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="SUBNET",type="string",JSONPath=".spec.subnet"
// +kubebuilder:printcolumn:name="IPADDR",type="string",JSONPath=".spec.ipAddr"
// +kubebuilder:printcolumn:name="MACADDR",type="string",JSONPath=".spec.macAddr"
// +kubebuilder:printcolumn:name="VALID",type="string",JSONPath=".status.valid"

type BindingIp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec BindingIpSpec `json:"spec"`
	Status BindingIpStatus `json:"status,omitempty"`
}

type BindingIpSpec struct {
	// subnet name (required)
	// +kubebuilder:validation:Required
	Subnet         string            `json:"subnet"`

	// IP address (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	IpAddr         string            `json:"ipAddr"`

	// Mac address (required)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9a-fA-F]{2}:){5}([0-9a-fA-F]{2})$`
	MacAddr        string            `json:"macAddr"`
}

type BindingIpStatus struct {
	// taking effect
	Valid bool `json:"valid"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BindingIpList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []BindingIp `json:"items"`
}
