package ssh

import (
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/infrastructure-io/topohub/pkg/clients/pool"
	"github.com/infrastructure-io/topohub/pkg/log"
)

type SSHSessionConfig struct {
	// IPAddr is the IP address of the ssh server, required.
	IPAddr string
	// Port is the port of the ssh server. Default is 22.
	Port int
	// Username is the username of the ssh server, required.
	Username string
	// Password is the password of the ssh server, required if SSHKeyAuth is false.
	Password string
	// SSHKey is the private key of the ssh server, required if SSHKeyAuth is true.
	SSHKey string
	// SSHKeyAuth is true if the ssh server uses private key authentication. Default is false.
	SSHKeyAuth bool
}

func (c *SSHSessionConfig) VerifyAndSetDefault() error {
	if c.IPAddr == "" {
		return fmt.Errorf("ip addr must not be empty")
	}
	if c.Port == 0 {
		c.Port = 22
	}
	if c.Username == "" {
		return fmt.Errorf("username must not be empty")
	}
	return nil
}

func (c *SSHSessionConfig) BuildAuthMethod() (ssh.AuthMethod, error) {
	var authMethod ssh.AuthMethod
	switch {
	case c.SSHKeyAuth && c.SSHKey != "":
		signer, err := ssh.ParsePrivateKey([]byte(c.SSHKey))
		if err != nil {
			return nil, fmt.Errorf("parse private key failed, err: %v", err)
		}
		authMethod = ssh.PublicKeys(signer)
	case c.Password != "":
		authMethod = ssh.Password(c.Password)
	default:
		return nil, fmt.Errorf("no valid authentication method provided")
	}
	return authMethod, nil
}

// NewSSHCLientOperations creates a new ssh client operations.
// Example for create a ssh client session pool.
//
//	pool := pool.NewSessionPool(ctx, NewSSHCLientOperations(nil))
func NewSSHCLientOperations(logger *zap.SugaredLogger) *SSHCLientOperations {
	if logger == nil {
		logger = log.Logger.Named("sshClientOperations")
	}
	return &SSHCLientOperations{
		log: logger,
	}
}

// Check implantation
var _ pool.ClientOperations[Client] = (*SSHCLientOperations)(nil)

type SSHCLientOperations struct {
	cfg *SSHSessionConfig
	log *zap.SugaredLogger
}

func (o *SSHCLientOperations) NewClient(cfg any) (Client, error) {
	sshCfg, err := verifySSHSessionConfig(cfg)
	if err != nil {
		return nil, err
	}
	cli, err := newSSHClient(sshCfg)
	if err != nil {
		return nil, err
	}
	o.cfg = sshCfg
	return cli, nil
}

func (SSHCLientOperations) Ping(client Client) error {
	return client.Ping()
}

func (SSHCLientOperations) Compare(old, new any) bool {
	return reflect.DeepEqual(old, new)
}

func (o *SSHCLientOperations) Refresh(oldClient Client, cfg any) (Client, error) {
	newcli, err := o.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if err := oldClient.Close(); err != nil {
		o.log.Warnf("close old ssh client failed, err: %v", err)
	}
	return newcli, nil
}

func (SSHCLientOperations) Close(client Client) error {
	return client.Close()
}

func verifySSHSessionConfig(cfg any) (*SSHSessionConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("session config must not be nil")
	}
	sshCfg, ok := cfg.(*SSHSessionConfig)
	if !ok {
		return nil, fmt.Errorf("invalid ssh session config")
	}
	if err := sshCfg.VerifyAndSetDefault(); err != nil {
		return nil, err
	}
	return sshCfg, nil
}

func newSSHClient(cfg *SSHSessionConfig) (Client, error) {
	authMethod, err := cfg.BuildAuthMethod()
	if err != nil {
		return nil, fmt.Errorf("build ssh auth method failed: %w", err)
	}
	config := &ssh.ClientConfig{
		User: cfg.Username,
		Auth: []ssh.AuthMethod{authMethod},
		// In production, a more secure method should be used
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	cli, err := NewClient(cfg.IPAddr, cfg.Port, config)
	if err != nil {
		return nil, fmt.Errorf("create ssh client failed: %w", err)
	}
	return cli, nil
}

func GenSessionID(cfg *SSHSessionConfig) string {
	if cfg == nil {
		return ""
	}
	return fmt.Sprintf("%s@%s:%d", cfg.Username, cfg.IPAddr, cfg.Port)
}
