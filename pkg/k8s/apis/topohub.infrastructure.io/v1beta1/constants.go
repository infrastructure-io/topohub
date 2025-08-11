package v1beta1

// Constants for labels
const (
	LabelIPAddr           = GroupName + "/ipAddr"
	LabelClientMode       = GroupName + "/mode"
	LabelClientActive     = GroupName + "/dhcp-ip-active"
	LabelClusterName      = GroupName + "/cluster-name"
	LabelSubnetName       = GroupName + "/subnet-name"
	LabelSourceType       = GroupName + "/source-type"
	LabelSecretCredential = GroupName + "/secret-credential"

	HostTypeDHCP     = "dhcp"
	HostTypeEndpoint = "hostendpoint"
	SSH              = "ssh"
	Redfish          = "redfish"
)

// Constants for PXE boot types
const (
	// PxeBootTypeIPMI represents IPMI PXE boot type
	PxeBootTypeIPMI = "ipmi"
	// PxeBootTypeRedfish represents Redfish PXE boot type
	PxeBootTypeRedfish = "redfish"
)

// Constants for annotations
const (
	// AnnotationBasicInfo 是用于存储 BasicInfo 结构体 JSON 的 annotation key
	AnnotationBasicInfo = GroupName + "/basicinfo"
)
