package data

import (
	"sync"

	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// SSHConnectCon stores the information required for SSH connection
type SSHConnectCon struct {
	Info       *topohubv1beta1.SSHBasicInfo
	Username   string
	Password   string
	SSHKey     string
	SSHKeyAuth bool
}

// SSHHostCache is used to cache SSH host connection information
type SSHHostCache struct {
	mu    sync.RWMutex
	hosts map[string]SSHConnectCon
}

// NewSSHHostCache creates a new SSH host cache
func NewSSHHostCache() *SSHHostCache {
	return &SSHHostCache{
		hosts: make(map[string]SSHConnectCon),
	}
}

// Add adds or updates SSH host connection information
func (c *SSHHostCache) Add(name string, con SSHConnectCon) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hosts[name] = con
}

// Get retrieves SSH host connection information
func (c *SSHHostCache) Get(name string) *SSHConnectCon {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if con, ok := c.hosts[name]; ok {
		return &con
	}
	return nil
}

// Delete removes SSH host connection information
func (c *SSHHostCache) Delete(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.hosts, name)
}

// GetAll retrieves all SSH host connection information
func (c *SSHHostCache) GetAll() map[string]SSHConnectCon {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]SSHConnectCon, len(c.hosts))
	for k, v := range c.hosts {
		result[k] = v
	}
	return result
}

// UpdateSecret updates the username and password for the specified Secret, returns a list of affected hosts
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
	if len(changedHosts) > 0 {
		log.Logger.Infof("update ssh status username for host %v", changedHosts)
	}
	return changedHosts
}

// UpdateSSHKey updates the SSH key for the specified Secret, returns a list of affected hosts
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

// SSHCacheDatabase is the global SSH host cache instance
var SSHCacheDatabase = NewSSHHostCache()
