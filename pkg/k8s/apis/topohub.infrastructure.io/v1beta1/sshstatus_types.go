package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HostTypeSSH = "ssh"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="CLUSTERNAME",type="string",JSONPath=".status.basic.clusterName"
// +kubebuilder:printcolumn:name="HEALTHY",type="boolean",JSONPath=".status.healthy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type SSHStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status *SSHStatusStatus `json:"status,omitempty"`
}

type SSHStatusStatus struct {
	Healthy        bool              `json:"healthy"`
	LastUpdateTime string            `json:"lastUpdateTime"`
	Basic          SSHBasicInfo      `json:"basic"`
	Info           map[string]string `json:"info"`
}

// SSHBasicInfo incluse SSH connection basic info
type SSHBasicInfo struct {
	ClusterName string `json:"clusterName"`
	// SubnetName is the name of the subnet this host belongs to
	// +optional
	SubnetName *string `json:"subnetName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SSHStatusList include SSHStatus objects
type SSHStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SSHStatus `json:"items"`
}
