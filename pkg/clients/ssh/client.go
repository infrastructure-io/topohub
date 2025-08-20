package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/infrastructure-io/topohub/pkg/log"
)

type Client interface {
	// Ping checks if the SSH connection is alive
	Ping() error

	// Run executes a command on the remote host
	Run(cmd string) (string, error)

	// GetSystemInfo retrieves system information
	GetSystemInfo() (map[string]string, error)

	// Close closes the SSH connection
	Close() error
}

// Check implantation
var _ Client = (*clientImpl)(nil)

func NewClient(ip string, port int, cfg *ssh.ClientConfig) (Client, error) {
	// Build connection address
	addr := net.JoinHostPort(ip, strconv.Itoa(int(port)))
	// Establish SSH connection
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial ssh connection failed, err: %v", err)
	}
	return &clientImpl{conn: conn}, nil
}

type clientImpl struct {
	conn *ssh.Client
}

func (c *clientImpl) Ping() error {
	_, err := c.Run("echo 1")
	return err
}

func (c *clientImpl) Run(cmd string) (string, error) {
	if c.conn == nil {
		return "", errors.New("conn is nil")
	}
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("new ssh session failed: %w", err)
	}
	defer func() {
		if err := session.Close(); err != nil && err != io.EOF {
			log.Logger.Errorf("close ssh session failed: %v", err)
		}
	}()

	res, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(res), err
	}
	return string(res), nil
}

func (c *clientImpl) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// GetSystemInfo retrieves system information
func (c *clientImpl) GetSystemInfo() (map[string]string, error) {
	info := make(map[string]string)
	var errs []error

	// get hostname
	hostname, err := c.Run("hostname")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get hostname: %v", err))
	} else {
		info["Hostname"] = strings.TrimSpace(hostname)
	}

	// get os info
	osInfo, err := c.Run("cat /etc/os-release | grep PRETTY_NAME | cut -d '\"' -f 2")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get OS info: %v", err))
	} else {
		info["OS"] = strings.TrimSpace(osInfo)
	}

	// get kernel version
	kernel, err := c.Run("uname -r")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get kernel version: %v", err))
	} else {
		info["Kernel"] = strings.TrimSpace(kernel)
	}

	// get CPU info
	cpuInfo, err := c.Run("cat /proc/cpuinfo | grep 'model name' | head -1 | cut -d ':' -f 2")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get CPU info: %v", err))
	} else {
		info["CPU"] = strings.TrimSpace(cpuInfo)
	}

	// get CPU cores
	cpuCores, err := c.Run("nproc")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get CPU cores: %v", err))
	} else {
		info["CPUCores"] = strings.TrimSpace(cpuCores)
	}

	// get memory info
	memInfo, err := c.Run("free -h | grep Mem | awk '{print $2}'")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get memory info: %v", err))
	} else {
		info["Memory"] = strings.TrimSpace(memInfo)
	}

	// get GPU count
	gpuCount, err := c.Run("lspci | grep Display | wc -l")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get GPU count: %v", err))
	} else {
		info["GPUCount"] = strings.TrimSpace(gpuCount)
	}

	// get network info
	netInfo, err := c.Run("lspci -v | grep -i 'ethernet controller' | grep -vi 'virtual function'")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get network info: %v", err))
	} else {
		info["Network"] = strings.TrimSpace(netInfo)
	}

	// get storage info
	storageInfo, err := c.Run(`lsblk -d -o NAME,SIZE,TYPE,TRAN | grep -E 'disk|nvme' | grep -v 'loop\|rom' | awk '{print $1,$2}'`)
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
