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

// NewClient 创建一个新的SSH客户端
func NewClient(hostInfo sshstatusdata.SSHConnectCon, logger *zap.SugaredLogger) (*Client, error) {
	if hostInfo.Info == nil {
		return nil, fmt.Errorf("host info is nil")
	}

	authMethods := []ssh.AuthMethod{}

	// 根据认证方式选择不同的认证方法
	if hostInfo.SSHKeyAuth && hostInfo.SSHKey != "" {
		// 使用SSH密钥认证
		signer, err := ssh.ParsePrivateKey([]byte(hostInfo.SSHKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if hostInfo.Username != "" && hostInfo.Password != "" {
		// 使用密码认证
		authMethods = append(authMethods, ssh.Password(hostInfo.Password))
	} else {
		return nil, fmt.Errorf("no valid authentication method provided")
	}

	config := &ssh.ClientConfig{
		User:            hostInfo.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 在生产环境中应该使用更安全的方式
		Timeout:         10 * time.Second,
	}

	// 构建连接地址
	addr := net.JoinHostPort(hostInfo.Info.IpAddr, strconv.Itoa(int(hostInfo.Info.Port)))

	// 建立SSH连接
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

// Close 关闭SSH连接
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RunCommand 在远程主机上执行命令并返回输出
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

// GetSystemInfo 获取系统信息
func (c *Client) GetSystemInfo() (map[string]string, error) {
	info := make(map[string]string)

	// 获取主机名
	hostname, err := c.RunCommand("hostname")
	if err == nil {
		info["Hostname"] = strings.TrimSpace(hostname)
	}

	// 获取操作系统信息
	osInfo, err := c.RunCommand("cat /etc/os-release | grep PRETTY_NAME | cut -d '\"' -f 2")
	if err == nil {
		info["OS"] = strings.TrimSpace(osInfo)
	}

	// 获取内核版本
	kernel, err := c.RunCommand("uname -r")
	if err == nil {
		info["Kernel"] = strings.TrimSpace(kernel)
	}

	// 获取CPU信息
	cpuInfo, err := c.RunCommand("cat /proc/cpuinfo | grep 'model name' | head -1 | cut -d ':' -f 2")
	if err == nil {
		info["CPU"] = strings.TrimSpace(cpuInfo)
	}

	// 获取CPU核心数
	cpuCores, err := c.RunCommand("nproc")
	if err == nil {
		info["CPUCores"] = strings.TrimSpace(cpuCores)
	}

	// 获取内存信息
	memInfo, err := c.RunCommand("free -h | grep Mem | awk '{print $2}'")
	if err == nil {
		info["Memory"] = strings.TrimSpace(memInfo)
	}

	// 获取GPU个数
	gpuCount, err := c.RunCommand("lspci | grep Display | wc -l")
	if err == nil {
		info["GPUCount"] = strings.TrimSpace(gpuCount)
	}

	// 获取物理网卡信息
	netInfo, err := c.RunCommand("lspci -v | grep -i 'ethernet controller' | grep -vi 'virtual function'")
	if err == nil {
		info["Network"] = strings.TrimSpace(netInfo)
	}

	// 获取物理存储信息
	storageInfo, err := c.RunCommand(`lsblk -d -o NAME,SIZE,TYPE,TRAN | grep -E 'disk|nvme' | grep -v 'loop\|rom' | awk '{print $1,$2}'`)
	if err == nil {
		info["Storage"] = strings.TrimSpace(storageInfo)
	}

	return info, nil
}

// GetSystemLogs 获取系统日志
func (c *Client) GetSystemLogs(lines int) ([]map[string]string, error) {
	cmd := fmt.Sprintf("journalctl -n %d --no-pager", lines)
	output, err := c.RunCommand(cmd)
	if err != nil {
		return nil, err
	}

	logLines := strings.Split(output, "\n")
	logs := make([]map[string]string, 0, len(logLines))

	for _, line := range logLines {
		if line == "" {
			continue
		}

		// 解析日志行，提取时间、级别和消息
		parts := strings.SplitN(line, " ", 4)
		if len(parts) < 4 {
			continue
		}

		timestamp := strings.Join(parts[:3], " ")
		message := parts[3]

		// 确定日志级别
		level := "INFO"
		if strings.Contains(strings.ToLower(message), "error") || strings.Contains(strings.ToLower(message), "fail") {
			level = "ERROR"
		} else if strings.Contains(strings.ToLower(message), "warn") {
			level = "WARNING"
		}

		logs = append(logs, map[string]string{
			"timestamp": timestamp,
			"level":     level,
			"message":   message,
		})
	}

	return logs, nil
}

// IsHealthy 检查SSH连接是否健康
func (c *Client) IsHealthy() bool {
	// 尝试执行一个简单的命令来检查连接是否正常
	_, err := c.RunCommand("echo 'Connection test'")
	return err == nil
}
