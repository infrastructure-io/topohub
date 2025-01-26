package v1beta1

import (
	"github.com/stmcginnis/gofish/redfish"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	HostOperationStatusPending = "pending"
	HostOperationStatusSuccess = "success"
	HostOperationStatusFailed  = "failed"
)

const (
	// operation action
	// power
	// "On"
	BootCmdOn = string(redfish.OnResetType)
	// "ForceOn"
	BootCmdForceOn = string(redfish.ForceOnResetType)
	// "ForceOff"
	BootCmdForceOff = string(redfish.ForceOffResetType)
	// "GracefulShutdown"
	BootCmdGracefulShutdown = string(redfish.GracefulShutdownResetType)
	// "ForceRestart"
	BootCmdForceRestart = string(redfish.ForceRestartResetType)
	// "GracefulRestart"
	BootCmdGracefulRestart = string(redfish.GracefulRestartResetType)
	// "PxeReboot"
	BootCmdResetPxeOnce string = "PxeReboot"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="ACTION",type="string",JSONPath=".spec.action"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="CLUSTERAGENT",type="string",JSONPath=".status.clusterAgent"
// +kubebuilder:printcolumn:name="HOSTIP",type="string",JSONPath=".status.ipAddr"

type HostOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostOperationSpec   `json:"spec,omitempty"`
	Status HostOperationStatus `json:"status,omitempty"`
}

type HostOperationSpec struct {
	// +kubebuilder:validation:Enum=ForceOn;On;ForceOff;GracefulShutdown;ForceRestart;GracefulRestart;PxeReboot
	// +kubebuilder:validation:Required
	Action string `json:"action"`

	// +kubebuilder:validation:Required
	HostStatusName string `json:"hostStatusName"`
}

type HostOperationStatus struct {
	// +kubebuilder:validation:Enum=pending;success;failure
	Status string `json:"status,omitempty"`

	Message string `json:"message,omitempty"`

	LastUpdateTime string `json:"lastUpdateTime,omitempty"`

	ClusterAgent string `json:"clusterAgent,omitempty"`

	IpAddr string `json:"ipAddr,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HostOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HostOperation `json:"items"`
}
