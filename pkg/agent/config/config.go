package config

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/infrastructure-io/topohub/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	bmcv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
)

// AgentConfig represents the agent configuration
type AgentConfig struct {
	// 集群代理名称
	ClusterAgentName string
	AgentObjSpec     bmcv1beta1.ClusterAgentSpec
	// redfish 连接的缺省用户名
	Username string
	// redfish 连接的缺省密码
	Password string
	// 主机状态更新间隔（秒）
	HostStatusUpdateInterval int
	// pod namespace
	PodNamespace string
}

// ValidateEndpointConfig validates the endpoint configuration
func (c *AgentConfig) ValidateEndpointConfig(clientset *kubernetes.Clientset) error {
	if c.AgentObjSpec.Endpoint == nil {
		return fmt.Errorf("endpoint configuration is required")
	}

	// Check if port is valid
	if c.AgentObjSpec.Endpoint.Port <= 0 || c.AgentObjSpec.Endpoint.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.AgentObjSpec.Endpoint.Port)
	}

	// Get credentials from secret if specified
	if c.AgentObjSpec.Endpoint.SecretName != "" && c.AgentObjSpec.Endpoint.SecretNamespace != "" {
		// Get the secret
		secret, err := clientset.CoreV1().Secrets(c.AgentObjSpec.Endpoint.SecretNamespace).Get(
			context.TODO(),
			c.AgentObjSpec.Endpoint.SecretName,
			metav1.GetOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to get credentials secret: %v", err)
		}

		// Extract username and password
		username, ok := secret.Data["username"]
		if !ok {
			return fmt.Errorf("username not found in secret")
		}
		password, ok := secret.Data["password"]
		if !ok {
			return fmt.Errorf("password not found in secret")
		}

		// Store credentials in config
		c.Username = string(username)
		c.Password = string(password)

		log.Logger.Debugf("Successfully loaded credentials from secret %s/%s",
			c.AgentObjSpec.Endpoint.SecretNamespace,
			c.AgentObjSpec.Endpoint.SecretName)
	}

	return nil
}

// ValidateFeatureConfig validates the feature configuration
func (c *AgentConfig) ValidateFeatureConfig() error {
	if c.AgentObjSpec.Feature == nil {
		return fmt.Errorf("feature configuration is required")
	}

	if c.AgentObjSpec.Feature.EnableDhcpServer {
		if c.AgentObjSpec.Feature.DhcpServerConfig == nil {
			return fmt.Errorf("dhcp server config must be specified when dhcp server is enabled")
		}

		config := c.AgentObjSpec.Feature.DhcpServerConfig

		if config.DhcpServerInterface == "" {
			return fmt.Errorf("dhcp server interface must be specified when dhcp server is enabled")
		}

		// Check if interface exists
		_, err := net.InterfaceByName(config.DhcpServerInterface)
		if err != nil {
			return fmt.Errorf("dhcp server interface %s not found: %v", config.DhcpServerInterface, err)
		}
	}

	return nil
}

// GetDetailString returns a detailed string representation of the AgentConfig
func (c *AgentConfig) GetDetailString() string {
	var details strings.Builder

	details.WriteString(fmt.Sprintf("ClusterAgentName: %s\n", c.ClusterAgentName))
	details.WriteString("AgentSpec:\n")

	// AgentYaml details
	details.WriteString("  AgentYaml:\n")
	details.WriteString(fmt.Sprintf("    UnderlayInterface: %s\n", c.AgentObjSpec.AgentYaml.UnderlayInterface))
	details.WriteString(fmt.Sprintf("    Image: %s\n", c.AgentObjSpec.AgentYaml.Image))
	if c.AgentObjSpec.AgentYaml.Replicas != nil {
		details.WriteString(fmt.Sprintf("    Replicas: %d\n", *c.AgentObjSpec.AgentYaml.Replicas))
	}
	if c.AgentObjSpec.AgentYaml.NodeName != "" {
		details.WriteString(fmt.Sprintf("    NodeName: %s\n", c.AgentObjSpec.AgentYaml.NodeName))
	}

	// Endpoint details
	if c.AgentObjSpec.Endpoint != nil {
		details.WriteString("  Endpoint:\n")
		details.WriteString(fmt.Sprintf("    Port: %d\n", c.AgentObjSpec.Endpoint.Port))
		details.WriteString(fmt.Sprintf("    HTTPS: %v\n", c.AgentObjSpec.Endpoint.HTTPS))
		if c.AgentObjSpec.Endpoint.SecretName != "" {
			details.WriteString(fmt.Sprintf("    SecretName: %s\n", c.AgentObjSpec.Endpoint.SecretName))
		}
		if c.AgentObjSpec.Endpoint.SecretNamespace != "" {
			details.WriteString(fmt.Sprintf("    SecretNamespace: %s\n", c.AgentObjSpec.Endpoint.SecretNamespace))
		}
		details.WriteString(fmt.Sprintf("    Username: %v\n", c.Username != ""))
		details.WriteString(fmt.Sprintf("    Password: %v\n", c.Password != ""))
	}

	// Feature details
	if c.AgentObjSpec.Feature != nil {
		details.WriteString("  Feature:\n")
		details.WriteString(fmt.Sprintf("    EnableDhcpServer: %v\n", c.AgentObjSpec.Feature.EnableDhcpServer))

		// DHCP Server Config details
		if c.AgentObjSpec.Feature.DhcpServerConfig != nil {
			details.WriteString("    DhcpServerConfig:\n")
			config := c.AgentObjSpec.Feature.DhcpServerConfig
			details.WriteString(fmt.Sprintf("      EnableDhcpDiscovery: %v\n", config.EnableDhcpDiscovery))
			details.WriteString(fmt.Sprintf("      EnableBindDhcpIP: %v\n", config.EnableBindDhcpIP))
			details.WriteString(fmt.Sprintf("      EnableBindStaticIP: %v\n", config.EnableBindStaticIP))
			details.WriteString(fmt.Sprintf("      DhcpServerInterface: %s\n", config.DhcpServerInterface))
			details.WriteString(fmt.Sprintf("      Subnet: %s\n", config.Subnet))
			details.WriteString(fmt.Sprintf("      IpRange: %s\n", config.IpRange))
			details.WriteString(fmt.Sprintf("      Gateway: %s\n", config.Gateway))
			if config.SelfIp != "" {
				details.WriteString(fmt.Sprintf("      SelfIp: %s\n", config.SelfIp))
			}
		}
	}

	// Add HostStatusUpdateInterval to details
	details.WriteString(fmt.Sprintf("  HostStatusUpdateInterval: %d seconds\n", c.HostStatusUpdateInterval))

	return details.String()
}

// LoadAgentConfig loads the agent configuration from environment and ClusterAgent instance
// environment variable:
// CLUSTERAGENT_NAME: the name of the ClusterAgent
// HOST_STATUS_UPDATE_INTERVAL: the interval of updating host status, default is 60 seconds
func LoadAgentConfig(k8sClient *kubernetes.Clientset) (*AgentConfig, error) {
	// Get agent name from environment
	agentName := os.Getenv("CLUSTERAGENT_NAME")
	if agentName == "" {
		return nil, fmt.Errorf("CLUSTERAGENT_NAME environment variable not set")
	}

	ns := os.Getenv("POD_NAMESPACE")

	updateInterval := 60 // 默认 60 秒
	intervalStr := os.Getenv("HOST_STATUS_UPDATE_INTERVAL")
	if intervalStr == "" {
		return nil, fmt.Errorf("HOST_STATUS_UPDATE_INTERVAL environment variable is not set, using default: %d", updateInterval)
	} else {
		if interval, err := strconv.Atoi(intervalStr); err == nil {
			updateInterval = interval
		} else {
			return nil, fmt.Errorf("HOST_STATUS_UPDATE_INTERVAL environment variable %s is not a valid integer: %v", intervalStr, err)
		}
	}

	// Create bmc client config
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	// Add bmc scheme and set GroupVersion
	scheme := runtime.NewScheme()
	err = bmcv1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to add bmc scheme: %v", err)
	}
	restConfig.GroupVersion = &bmcv1beta1.SchemeGroupVersion
	restConfig.APIPath = "/apis"
	restConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme)

	// Create REST client for ClusterAgent
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %v", err)
	}

	// Get ClusterAgent instance
	clusterAgent := &bmcv1beta1.ClusterAgent{}
	err = restClient.Get().
		Resource("clusteragents").
		Name(agentName).
		Do(context.TODO()).
		Into(clusterAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to get ClusterAgent instance: %v", err)
	}

	// Create agent config
	agentConfig := &AgentConfig{
		ClusterAgentName:         agentName,
		AgentObjSpec:             clusterAgent.Spec,
		HostStatusUpdateInterval: updateInterval,
		PodNamespace:             ns,
	}

	// Validate endpoint configuration
	if err := agentConfig.ValidateEndpointConfig(k8sClient); err != nil {
		return nil, fmt.Errorf("invalid endpoint configuration: %v", err)
	}

	// Validate feature configuration
	if err := agentConfig.ValidateFeatureConfig(); err != nil {
		return nil, fmt.Errorf("invalid feature configuration: %v", err)
	}

	log.Logger.Debugf("Agent configuration loaded successfully: %+v", agentConfig)
	return agentConfig, nil
}
