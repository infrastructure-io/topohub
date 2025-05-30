package v1beta1

// Constants for labels
const (
	LabelIPAddr       = GroupName + "/ipAddr"
	LabelClientMode   = GroupName + "/mode"
	LabelClientActive = GroupName + "/dhcp-ip-active"
	LabelClusterName  = GroupName + "/cluster-name"
	LabelSubnetName   = GroupName + "/subnet-name"

	HostTypeDHCP     = "dhcp"
	HostTypeEndpoint = "hostendpoint"
)
