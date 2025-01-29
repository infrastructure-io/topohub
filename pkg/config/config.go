package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/infrastructure-io/topohub/pkg/log"
	"github.com/infrastructure-io/topohub/pkg/tools"
)

// AgentConfig represents the agent configuration
type AgentConfig struct {

	// pod namespace
	PodNamespace string

	// node name
	NodeName string

	// webhook cert dir
	WebhookCertDir string

	// storage path
	StoragePath           string
	StoragePathDhcpLog    string
	StoragePathDhcpLease  string
	StoragePathDhcpConfig string
	StoragePathZtp        string
	StoragePathSftp       string
	StoragePathHttp       string

	// dnsmasq config template path
	DhcpConfigTemplatePath string

	// FeatureConfigPath is the path to the feature configuration file
	FeatureConfigPath string
	// Redfish configuration
	RedfishPort                     int
	RedfishHttps                    bool
	RedfishSecretName               string
	RedfishSecretNamespace          string
	RedfishHostStatusUpdateInterval int
	// DHCP server configuration
	DhcpServerInterface string
}

// LoadFeatureConfig loads feature configuration from the config file
func (c *AgentConfig) loadFeatureConfig() error {
	// Read redfishPort
	portBytes, err := os.ReadFile(filepath.Join(c.FeatureConfigPath, "redfishPort"))
	if err != nil {
		return fmt.Errorf("failed to read redfishPort: %v", err)
	}
	port, err := strconv.Atoi(string(portBytes))
	if err != nil {
		return fmt.Errorf("invalid redfishPort value: %v", err)
	}
	c.RedfishPort = port

	// Read redfishHttps
	httpsBytes, err := os.ReadFile(filepath.Join(c.FeatureConfigPath, "redfishHttps"))
	if err != nil {
		return fmt.Errorf("failed to read redfishHttps: %v", err)
	}
	c.RedfishHttps = string(httpsBytes) == "true"

	// Read redfishSecretname
	secretNameBytes, err := os.ReadFile(filepath.Join(c.FeatureConfigPath, "redfishSecretname"))
	if err != nil {
		return fmt.Errorf("failed to read redfishSecretname: %v", err)
	}
	c.RedfishSecretName = string(secretNameBytes)

	// Read redfishSecretNamespace
	secretNsBytes, err := os.ReadFile(filepath.Join(c.FeatureConfigPath, "redfishSecretNamespace"))
	if err != nil {
		return fmt.Errorf("failed to read redfishSecretNamespace: %v", err)
	}
	c.RedfishSecretNamespace = string(secretNsBytes)

	// Read redfishHostStatusUpdateInterval
	intervalBytes, err := os.ReadFile(filepath.Join(c.FeatureConfigPath, "redfishHostStatusUpdateInterval"))
	if err != nil {
		return fmt.Errorf("failed to read redfishHostStatusUpdateInterval: %v", err)
	}
	interval, err := strconv.Atoi(string(intervalBytes))
	if err != nil {
		return fmt.Errorf("invalid redfishHostStatusUpdateInterval value: %v", err)
	}
	c.RedfishHostStatusUpdateInterval = interval

	// Read dhcpServerInterface
	interfaceBytes, err := os.ReadFile(filepath.Join(c.FeatureConfigPath, "dhcpServerInterface"))
	if err != nil {
		return fmt.Errorf("failed to read dhcpServerInterface: %v", err)
	}
	c.DhcpServerInterface = string(interfaceBytes)
	// Validate interface exists on the system
	if err := tools.ValidateInterfaceExists(c.DhcpServerInterface); err != nil {
		return fmt.Errorf("failed to find dhcpServer Interface %s: %v", c.DhcpServerInterface, err)
	}

	return nil
}

// verifyWebhookCertDir verifies that the webhook certificate directory exists and contains required files
func (c *AgentConfig) verifyWebhookCertDir() error {
	requiredFiles := []string{"tls.crt", "tls.key", "ca.crt"}
	for _, file := range requiredFiles {
		path := filepath.Join(c.WebhookCertDir, file)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required webhook certificate file %s not found: %v", file, err)
		}
	}
	return nil
}

// ensureStoragePath ensures that the storage path exists
func (c *AgentConfig) ensureStoragePath() error {
	if err := os.MkdirAll(c.StoragePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage path %s: %v", c.StoragePath, err)
	}
	return nil
}

func (c *AgentConfig) initStorageDirectory() error {
	// Check if main storage directory exists
	if _, err := os.Stat(c.StoragePath); err != nil {
		return fmt.Errorf("did not exist storage path %s: %v", c.StoragePath, err)
	}

	c.StoragePathDhcpLease = filepath.Join(c.StoragePath, "dhcp/lease")
	c.StoragePathDhcpConfig = filepath.Join(c.StoragePath, "dhcp/config")
	c.StoragePathDhcpLog = filepath.Join(c.StoragePath, "dhcp/log")
	c.StoragePathZtp = filepath.Join(c.StoragePath, "ztp")
	c.StoragePathSftp = filepath.Join(c.StoragePath, "sftp")
	c.StoragePathHttp = filepath.Join(c.StoragePath, "http")

	// List of required subdirectories
	subdirs := []string{c.StoragePathDhcpLease, c.StoragePathDhcpConfig, c.StoragePathDhcpLog, c.StoragePathZtp, c.StoragePathSftp, c.StoragePathHttp}

	// Check and create each subdirectory if it doesn't exist
	for _, dir := range subdirs {
		subPath := filepath.Join(c.StoragePath, dir)
		if _, err := os.Stat(subPath); os.IsNotExist(err) {
			if err := os.MkdirAll(subPath, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory %s: %v", subPath, err)
			}
		}
	}

	return nil
}

func LoadAgentConfig() (*AgentConfig, error) {
	agentConfig := &AgentConfig{}

	// Load environment variables
	agentConfig.PodNamespace = os.Getenv("POD_NAMESPACE")
	if agentConfig.PodNamespace == "" {
		return nil, fmt.Errorf("POD_NAMESPACE environment variable not set")
	}

	agentConfig.NodeName = os.Getenv("NODE_NAME")
	if agentConfig.NodeName == "" {
		return nil, fmt.Errorf("NODE_NAME environment variable not set")
	}

	agentConfig.WebhookCertDir = os.Getenv("WEBHOOK_CERT_DIR")
	if agentConfig.WebhookCertDir == "" {
		return nil, fmt.Errorf("WEBHOOK_CERT_DIR environment variable not set")
	}

	agentConfig.StoragePath = os.Getenv("STORAGE_PATH")
	if agentConfig.StoragePath == "" {
		return nil, fmt.Errorf("STORAGE_PATH environment variable not set")
	}

	agentConfig.FeatureConfigPath = os.Getenv("FEATURE_CONFIG_PATH")
	if agentConfig.FeatureConfigPath == "" {
		return nil, fmt.Errorf("FEATURE_CONFIG_PATH environment variable not set")
	}

	agentConfig.DhcpConfigTemplatePath = os.Getenv("DHCP_CONFIG_TEMPLATE_PATH")
	if agentConfig.DhcpConfigTemplatePath == "" {
		return nil, fmt.Errorf("DHCP_CONFIG_TEMPLATE_PATH environment variable not set")
	}

	// Load feature configuration
	if err := agentConfig.loadFeatureConfig(); err != nil {
		return nil, fmt.Errorf("failed to load feature configuration: %v", err)
	}

	// Verify webhook certificate directory
	if err := agentConfig.verifyWebhookCertDir(); err != nil {
		return nil, fmt.Errorf("webhook certificate verification failed: %v", err)
	}

	// Ensure storage path exists
	if err := agentConfig.ensureStoragePath(); err != nil {
		return nil, fmt.Errorf("failed to ensure storage path: %v", err)
	}

	log.Logger.Info("Agent configuration loaded successfully")
	return agentConfig, nil
}
