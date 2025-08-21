package redfish

import (
	"errors"
	"testing"

	gomonkey "github.com/agiledragon/gomonkey/v2"
	"github.com/stmcginnis/gofish"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/infrastructure-io/topohub/pkg/log"
)

func TestRedfishSessionConfigVerifyAndSetDefault(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *RedfishSessionConfig
		wantErr string
		wantCfg *RedfishSessionConfig
	}{
		{
			name:    "empty ip addr",
			cfg:     &RedfishSessionConfig{},
			wantErr: "ip addr must not be empty",
		},
		{
			name: "empty username",
			cfg: &RedfishSessionConfig{
				IPAddr: "127.0.0.1",
			},
			wantErr: "username and password must not be empt",
		},
		{
			name: "empty password",
			cfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Username: "root",
			},
			wantErr: "username and password must not be empt",
		},
		{
			name: "verify successful and set default values 1",
			cfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Username: "root",
				Password: "123456",
			},
			wantCfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     80,
				Https:    false,
				Username: "root",
				Password: "123456",
			},
		},
		{
			name: "verify successful and set default values 2",
			cfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Https:    true,
				Username: "root",
				Password: "123456",
			},
			wantCfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     443,
				Https:    true,
				Username: "root",
				Password: "123456",
			},
		},
		{
			name: "verify successful",
			cfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     8443,
				Https:    true,
				Username: "root",
				Password: "123456",
			},
			wantCfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Port:     8443,
				Https:    true,
				Username: "root",
				Password: "123456",
			},
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

func TestRedfishSessionConfigURL(t *testing.T) {
	cases := []struct {
		name string
		cfg  *RedfishSessionConfig
		want string
	}{
		{
			name: "http",
			cfg: &RedfishSessionConfig{
				IPAddr: "127.0.0.1",
				Https:  false,
				Port:   80,
			},
			want: "http://127.0.0.1:80",
		},
		{
			name: "https",
			cfg: &RedfishSessionConfig{
				IPAddr: "127.0.0.1",
				Https:  true,
				Port:   443,
			},
			want: "https://127.0.0.1:443",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.cfg.URL())
		})
	}
}

func TestSessionID(t *testing.T) {
	cfg := RedfishSessionConfig{
		IPAddr:   "127.0.0.1",
		Username: "root",
		Port:     80,
	}
	assert.Equal(t, "root@127.0.0.1:80", cfg.SessionID())
}

func TestNewRedfishClientOperations(t *testing.T) {
	t.Run("default logger", func(t *testing.T) {
		ops := NewRedfishClientOperations(nil)
		require.NotNil(t, ops)
		require.NotNil(t, ops.log)
		assert.Equal(t, "redfishClientOperations", ops.log.Desugar().Name())
	})

	t.Run("custom logger", func(t *testing.T) {
		ops := NewRedfishClientOperations(log.Logger.Named("customLogger"))
		require.NotNil(t, ops)
		require.NotNil(t, ops.log)
		assert.Equal(t, "customLogger", ops.log.Desugar().Name())
	})
}

func TestVerifyRedfishSessionConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     any
		wantErr string
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: "redfish session config must not be nil",
		},
		{
			name:    "invalid config type",
			cfg:     RedfishSessionConfig{},
			wantErr: "invalid redfish session config",
		},
		{
			name: "verify session config successful",
			cfg: &RedfishSessionConfig{
				IPAddr:   "127.0.0.1",
				Username: "root",
				Password: "123456",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := verifyRedfishSessionConfig(c.cfg)
			if c.wantErr != "" {
				require.ErrorContains(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewRedfishClient(t *testing.T) {
	cfg := &RedfishSessionConfig{
		IPAddr:   "127.0.0.1",
		Https:    true,
		Port:     8443,
		Username: "root",
		Password: "123456",
	}

	t.Run("Connect redfish failed", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFuncReturn(gofish.Connect, nil, errors.New("test error"))

		_, err := newRedfishClient(cfg, nil)
		require.ErrorContains(t, err, "connect to redfish server failed")
	})

	t.Run("new redfish client successful", func(t *testing.T) {
		patches := gomonkey.NewPatches()
		defer patches.Reset()

		patches.ApplyFunc(gofish.Connect, func(config gofish.ClientConfig,
		) (c *gofish.APIClient, err error) {
			assert.Equal(t, "https://127.0.0.1:8443", config.Endpoint)
			assert.Equal(t, "root", config.Username)
			assert.Equal(t, "123456", config.Password)
			assert.True(t, config.Insecure)
			assert.True(t, config.ReuseConnections)
			return nil, nil
		})

		client, err := newRedfishClient(cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, client)
	})
}
