package ssh

import (
	"errors"
	"io"
	"testing"

	gomonkey "github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/infrastructure-io/topohub/pkg/log"
)

func TestSSHSessionConfigVerifyAndSetDefault(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *SSHSessionConfig
		wantErr string
		wantCfg *SSHSessionConfig
	}{
		{
			name:    "empty ip addr",
			cfg:     &SSHSessionConfig{},
			wantErr: "ip addr must not be empty",
		},
		{
			name:    "empty username",
			cfg:     &SSHSessionConfig{IPAddr: "127.0.0.1"},
			wantErr: "username must not be empty",
		},
		{
			name:    "verify successful and set default values",
			cfg:     &SSHSessionConfig{IPAddr: "127.0.0.1", Username: "root"},
			wantCfg: &SSHSessionConfig{IPAddr: "127.0.0.1", Username: "root", Port: 22},
		},
		{
			name:    "verify successful",
			cfg:     &SSHSessionConfig{IPAddr: "127.0.0.1", Username: "root", Port: 1022},
			wantCfg: &SSHSessionConfig{IPAddr: "127.0.0.1", Username: "root", Port: 1022},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.VerifyAndSetDefault()
			if c.wantErr != "" {
				require.ErrorContains(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantCfg, c.cfg)
		})
	}
}

func TestSSHSessionConfigBuildAuthMethod(t *testing.T) {
	t.Run("no valid authentication method provided", func(t *testing.T) {
		cfg := &SSHSessionConfig{
			IPAddr:   "127.0.0.1",
			Port:     22,
			Username: "root",
		}
		_, err := cfg.BuildAuthMethod()
		require.ErrorContains(t, err, "no valid authentication method provided")
	})

	t.Run("parse private key failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFuncReturn(ssh.ParsePrivateKey, nil, errors.New("test error"))

		cfg := &SSHSessionConfig{
			IPAddr:     "127.0.0.1",
			Port:       22,
			Username:   "root",
			SSHKeyAuth: true,
			SSHKey:     "xxxx",
		}
		_, err := cfg.BuildAuthMethod()
		require.ErrorContains(t, err, "parse private key failed")
	})

	t.Run("build public key auth method successful", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFuncReturn(ssh.ParsePrivateKey, &fakeSinger{}, nil)

		cfg := &SSHSessionConfig{
			IPAddr:     "127.0.0.1",
			Port:       22,
			Username:   "root",
			SSHKeyAuth: true,
			SSHKey:     "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDRa",
		}
		authMethod, err := cfg.BuildAuthMethod()
		require.NoError(t, err)
		require.NotNil(t, authMethod)
	})

	t.Run("build password auth method successful", func(t *testing.T) {
		cfg := &SSHSessionConfig{
			IPAddr:   "127.0.0.1",
			Port:     22,
			Username: "root",
			Password: "123456",
		}

		authMethod, err := cfg.BuildAuthMethod()
		require.NoError(t, err)
		require.NotNil(t, authMethod)
	})
}

func TestSSHSessionConfigSessionID(t *testing.T) {
	cfg := &SSHSessionConfig{
		IPAddr:   "127.0.0.1",
		Port:     22,
		Username: "root",
	}
	assert.Equal(t, "root@127.0.0.1:22", cfg.SessionID())
}

func TestNewSSHCLientOperations(t *testing.T) {
	t.Run("default logger", func(t *testing.T) {
		ops := NewSSHCLientOperations(nil)
		require.NotNil(t, ops)
		require.NotNil(t, ops.log)
		assert.Equal(t, "sshClientOperations", ops.log.Desugar().Name())
	})

	t.Run("custom logger", func(t *testing.T) {
		ops := NewSSHCLientOperations(log.Logger.Named("customLogger"))
		require.NotNil(t, ops)
		require.NotNil(t, ops.log)
		assert.Equal(t, "customLogger", ops.log.Desugar().Name())
	})
}

func TestSSHCLientOperationsCompare(t *testing.T) {
	cases := []struct {
		name   string
		newcfg SSHSessionConfig
		want   bool
	}{
		{
			name: "configs are the same",
			newcfg: SSHSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     22,
				Username: "root",
				Password: "111111",
			},
			want: true,
		},
		{
			name: "IP addr was changed",
			newcfg: SSHSessionConfig{
				IPAddr:   "1.1.1.1",
				Port:     22,
				Username: "root",
				Password: "111111",
			},
			want: false,
		},
		{
			name: "Port was changed",
			newcfg: SSHSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     1022,
				Username: "root",
				Password: "111111",
			},
			want: false,
		},
		{
			name: "Username was changed",
			newcfg: SSHSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     22,
				Username: "user",
				Password: "111111",
			},
			want: false,
		},
		{
			name: "password was changed",
			newcfg: SSHSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     22,
				Username: "root",
				Password: "123456",
			},
			want: false,
		},
	}

	oldcfg := SSHSessionConfig{
		IPAddr:   "127.0.0.1",
		Port:     22,
		Username: "root",
		Password: "111111",
	}
	cliops := NewSSHCLientOperations(nil)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := cliops.Compare(oldcfg, c.newcfg)
			assert.Equal(t, c.want, b)
		})
	}
}

func TestVerifySSHSessionConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     any
		wantErr string
	}{
		{
			name:    "empty config",
			cfg:     nil,
			wantErr: "session config must not be nil",
		},
		{
			name:    "invalid config type",
			cfg:     SSHSessionConfig{},
			wantErr: "invalid ssh session config",
		},
		{
			name:    "verify successful",
			cfg:     &SSHSessionConfig{IPAddr: "127.0.0.1", Username: "root"},
			wantErr: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := verifySSHSessionConfig(c.cfg)
			if c.wantErr != "" {
				require.ErrorContains(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, cfg)
		})
	}
}

var _ ssh.Signer = (*fakeSinger)(nil)

type fakeSinger struct{}

func (fakeSinger) PublicKey() ssh.PublicKey {
	return nil
}

func (fakeSinger) Sign(_rand io.Reader, _data []byte) (*ssh.Signature, error) {
	return nil, nil
}

func TestNewSSHClient(t *testing.T) {
	t.Run("build auth method failed", func(t *testing.T) {
		sshCfg := &SSHSessionConfig{
			IPAddr:   "127.0.0.1",
			Port:     22,
			Username: "root",
			Password: "123456",
		}

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyMethodReturn((*SSHSessionConfig)(nil), "BuildAuthMethod",
			nil, errors.New("test error"))
		patches.ApplyFunc(NewClient, func(ip string, port int, cfg *ssh.ClientConfig,
		) (Client, error) {
			assert.Equal(t, sshCfg.IPAddr, ip)
			assert.Equal(t, sshCfg.Port, port)
			assert.NotNil(t, cfg)
			assert.Equal(t, sshCfg.Username, cfg.User)
			assert.Len(t, cfg.Auth, 1)
			return &clientImpl{}, nil
		})

		_, err := newSSHClient(sshCfg)
		require.ErrorContains(t, err, "build ssh auth method failed")
	})

	t.Run("new client failed", func(t *testing.T) {
		sshCfg := &SSHSessionConfig{
			IPAddr:   "127.0.0.1",
			Port:     22,
			Username: "root",
			Password: "123456",
		}

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFuncReturn(NewClient, nil, errors.New("test error"))

		_, err := newSSHClient(sshCfg)
		require.ErrorContains(t, err, "create ssh client failed")
	})

	t.Run("build auth method failed", func(t *testing.T) {
		sshCfg := &SSHSessionConfig{
			IPAddr:   "127.0.0.1",
			Port:     22,
			Username: "root",
			Password: "123456",
		}

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFunc(NewClient, func(ip string, port int, cfg *ssh.ClientConfig,
		) (Client, error) {
			assert.Equal(t, sshCfg.IPAddr, ip)
			assert.Equal(t, sshCfg.Port, port)
			assert.NotNil(t, cfg)
			assert.Equal(t, sshCfg.Username, cfg.User)
			assert.Len(t, cfg.Auth, 1)
			return &clientImpl{}, nil
		})

		cli, err := newSSHClient(sshCfg)
		require.NoError(t, err)
		require.NotNil(t, cli)
	})
}
