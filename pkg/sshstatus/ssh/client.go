package ssh

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	sshstatusdata "github.com/infrastructure-io/topohub/pkg/sshstatus/data"
)

type Client struct {
	conn     *ssh.Client
	config   *ssh.ClientConfig
	hostInfo *sshstatusdata.SSHConnectCon
	log      *zap.SugaredLogger
}

// NewClient creates a new SSH client
func NewClient(hostInfo sshstatusdata.SSHConnectCon, logger *zap.SugaredLogger) (*Client, error) {

	authMethods := []ssh.AuthMethod{}

	// Choose different authentication methods based on the configuration
	if hostInfo.SSHKeyAuth && hostInfo.SSHKey != "" {
		// Use SSH key authentication
		signer, err := ssh.ParsePrivateKey([]byte(hostInfo.SSHKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if hostInfo.Username != "" && hostInfo.Password != "" {
		// Use password authentication
		authMethods = append(authMethods, ssh.Password(hostInfo.Password))
	} else {
		return nil, fmt.Errorf("no valid authentication method provided")
	}

	config := &ssh.ClientConfig{
		User:            hostInfo.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, a more secure method should be used
		Timeout:         10 * time.Second,
	}

	// Build connection address
	addr := net.JoinHostPort(hostInfo.IPAddr, strconv.Itoa(int(hostInfo.Port)))

	// Establish SSH connection
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}

	return &Client{
		conn:     conn,
		config:   config,
		hostInfo: &hostInfo,
		log:      logger,
	}, nil
}

// Close terminates the SSH connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RunCommand executes a command on the remote host and returns the output
func (c *Client) RunCommand(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to run command: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// GetSystemInfo retrieves system information
func (c *Client) GetSystemInfo() (map[string]string, error) {
	info := make(map[string]string)
	var errs []error

	// get hostname
	hostname, err := c.RunCommand("hostname")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get hostname: %v", err))
	} else {
		info["Hostname"] = strings.TrimSpace(hostname)
	}

	// get os info
	osInfo, err := c.RunCommand("cat /etc/os-release | grep PRETTY_NAME | cut -d '\"' -f 2")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get OS info: %v", err))
	} else {
		info["OS"] = strings.TrimSpace(osInfo)
	}

	// get kernel version
	kernel, err := c.RunCommand("uname -r")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get kernel version: %v", err))
	} else {
		info["Kernel"] = strings.TrimSpace(kernel)
	}

	// get CPU info
	cpuInfo, err := c.RunCommand("cat /proc/cpuinfo | grep 'model name' | head -1 | cut -d ':' -f 2")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get CPU info: %v", err))
	} else {
		info["CPU"] = strings.TrimSpace(cpuInfo)
	}

	// get CPU cores
	cpuCores, err := c.RunCommand("nproc")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get CPU cores: %v", err))
	} else {
		info["CPUCores"] = strings.TrimSpace(cpuCores)
	}

	// get memory info
	memInfo, err := c.RunCommand("free -h | grep Mem | awk '{print $2}'")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get memory info: %v", err))
	} else {
		info["Memory"] = strings.TrimSpace(memInfo)
	}

	// get GPU count
	gpuCount, err := c.RunCommand("lspci | grep Display | wc -l")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get GPU count: %v", err))
	} else {
		info["GPUCount"] = strings.TrimSpace(gpuCount)
	}

	// get network info
	netInfo, err := c.RunCommand("lspci -v | grep -i 'ethernet controller' | grep -vi 'virtual function'")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get network info: %v", err))
	} else {
		info["Network"] = strings.TrimSpace(netInfo)
	}

	// get storage info
	storageInfo, err := c.RunCommand(`lsblk -d -o NAME,SIZE,TYPE,TRAN | grep -E 'disk|nvme' | grep -v 'loop\|rom' | awk '{print $1,$2}'`)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get storage info: %v", err))
	} else {
		info["Storage"] = strings.TrimSpace(storageInfo)
	}

	// If there are errors, return the collected errors
	if len(errs) > 0 {
		var errMsgs []string
		for _, e := range errs {
			errMsgs = append(errMsgs, e.Error())
		}
		return info, fmt.Errorf("partial system info collected, some errors occurred:\n%s",
			strings.Join(errMsgs, "\n"))
	}

	return info, nil
}

// IsHealthy checks if the SSH connection is healthy
func (c *Client) IsHealthy() bool {
	_, err := c.RunCommand("echo 'Connection test'")
	return err == nil
}
