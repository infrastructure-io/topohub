package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="CLUSTERAGENT",type="string",JSONPath=".spec.clusterAgent"
// +kubebuilder:printcolumn:name="HOSTIP",type="string",JSONPath=".spec.ipAddr"

// HostEndpoint is the Schema for the hostendpoints API
type HostEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HostEndpointSpec `json:"spec,omitempty"`
}

// HostEndpointSpec defines the desired state of HostEndpoint
type HostEndpointSpec struct {
	// ClusterAgent specifies which clusterAgent this hostEndpoint belongs to
	// +optional
	ClusterAgent string `json:"clusterAgent,omitempty"`

	// IPAddr is the IP address of the host endpoint
	// +kubebuilder:validation:Required
	IPAddr string `json:"ipAddr"`

	// SecretName is the name of the secret containing credentials
	// +optional
	SecretName *string `json:"secretName,omitempty"`

	// SecretNamespace is the namespace of the secret containing credentials
	// +optional
	SecretNamespace *string `json:"secretNamespace,omitempty"`

	// HTTPS specifies whether to use HTTPS for communication
	// +optional
	// +kubebuilder:default=true
	HTTPS *bool `json:"https,omitempty"`

	// Port specifies the port number for communication
	// +optional
	// +kubebuilder:default=443
	Port *int32 `json:"port,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostEndpointList contains a list of HostEndpoint
type HostEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostEndpoint `json:"items"`
}
