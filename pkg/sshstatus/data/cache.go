package data

// SSHConnectCon stores the information required for SSH connection
type SSHConnectCon struct {
	IPAddr     string
	Port       int
	Http       bool
	Username   string
	Password   string
	SSHKey     string // SSH private key for authentication
	SSHKeyAuth bool   // Whether to use SSH key authentication
}
