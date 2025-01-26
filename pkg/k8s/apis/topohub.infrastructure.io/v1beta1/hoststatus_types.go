package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelIPAddr       = GroupName + "/ipAddr"
	LabelClientMode   = GroupName + "/mode"
	LabelClientActive = GroupName + "/dhcp-ip-active"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="CLUSTERAGENT",type="string",JSONPath=".status.clusterAgent"
// +kubebuilder:printcolumn:name="HEALTHY",type="boolean",JSONPath=".status.healthy"
// +kubebuilder:printcolumn:name="IPADDR",type="string",JSONPath=".status.basic.ipAddr"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".status.basic.type"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type HostStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status HostStatusStatus `json:"status,omitempty"`
}

type HostStatusStatus struct {
	Healthy        bool              `json:"healthy"`
	ClusterAgent   string            `json:"clusterAgent"`
	LastUpdateTime string            `json:"lastUpdateTime"`
	Basic          BasicInfo         `json:"basic"`
	Info           map[string]string `json:"info"`
	Log            LogStruct         `json:"log"`
}

type LogStruct struct {
	// +kubebuilder:validation:Required
	TotalLogAccount   int32 `json:"totalLogAccount"`
	WarningLogAccount int32 `json:"warningLogAccount"`
	// +optional
	LastestLog *LogEntry `json:"lastestLog,omitempty"`
	// +optional
	LastestWarningLog *LogEntry `json:"lastestWarningLog,omitempty"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

type BasicInfo struct {
	Type            string `json:"type"`
	IpAddr          string `json:"ipAddr"`
	SecretName      string `json:"secretName"`
	SecretNamespace string `json:"secretNamespace"`
	Https           bool   `json:"https"`
	Port            int32  `json:"port"`
	Mac             string `json:"mac,omitempty"`
	// ActiveDhcpClient specifies this host is an active dhcp client when type is dhcp
	// +optional
	ActiveDhcpClient bool `json:"activeDhcpClient,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HostStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HostStatus `json:"items"`
}
