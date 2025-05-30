package data

import (
	"sync"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// SSHConnectCon 存储SSH连接所需的信息
type SSHConnectCon struct {
	Info     *topohubv1beta1.SSHBasicInfo
	Username string
	Password string
	SSHKey   string
	SSHKeyAuth bool
}

// SSHHostCache 用于缓存SSH主机连接信息
type SSHHostCache struct {
	mu    sync.RWMutex
	hosts map[string]SSHConnectCon
}

// NewSSHHostCache 创建一个新的SSH主机缓存
func NewSSHHostCache() *SSHHostCache {
	return &SSHHostCache{
		hosts: make(map[string]SSHConnectCon),
	}
}

// Add 添加或更新SSH主机连接信息
func (c *SSHHostCache) Add(name string, con SSHConnectCon) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hosts[name] = con
}

// Get 获取SSH主机连接信息
func (c *SSHHostCache) Get(name string) *SSHConnectCon {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if con, ok := c.hosts[name]; ok {
		return &con
	}
	return nil
}

// Delete 删除SSH主机连接信息
func (c *SSHHostCache) Delete(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.hosts, name)
}

// GetAll 获取所有SSH主机连接信息
func (c *SSHHostCache) GetAll() map[string]SSHConnectCon {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]SSHConnectCon, len(c.hosts))
	for k, v := range c.hosts {
		result[k] = v
	}
	return result
}

// UpdateSecret 更新指定Secret的用户名和密码，返回受影响的主机列表
func (c *SSHHostCache) UpdateSecret(secretName, secretNamespace, username, password string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	var changedHosts []string
	for name, con := range c.hosts {
		if con.Info.SecretName == secretName && con.Info.SecretNamespace == secretNamespace {
			con.Username = username
			con.Password = password
			c.hosts[name] = con
			changedHosts = append(changedHosts, name)
		}
	}
	return changedHosts
}

// UpdateSSHKey 更新指定Secret的SSH密钥，返回受影响的主机列表
func (c *SSHHostCache) UpdateSSHKey(secretName, secretNamespace, sshKey string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	var changedHosts []string
	for name, con := range c.hosts {
		if con.Info.SecretName == secretName && con.Info.SecretNamespace == secretNamespace && con.SSHKeyAuth {
			con.SSHKey = sshKey
			c.hosts[name] = con
			changedHosts = append(changedHosts, name)
		}
	}
	return changedHosts
}

// SSHCacheDatabase 是全局的SSH主机缓存实例
var SSHCacheDatabase = NewSSHHostCache()
