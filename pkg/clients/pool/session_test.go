package pool

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSession(t *testing.T) {
	const (
		config    = "{}"
		sessionID = "1000"
	)
	cli := &fakeClient{}
	cases := []struct {
		name       string
		sessionID  string
		operations ClientOperations[*fakeClient]
		wantErr    string
	}{
		{
			name:    "session id is empty",
			wantErr: "session id must not be empty",
		},
		{
			name:      "operations is nil",
			sessionID: sessionID,
			wantErr:   "operations must not be nil",
		},
		{
			name:      "new client failed",
			sessionID: sessionID,
			operations: &fakeClientOperation{
				wantError: map[string]error{
					"NewClient": errors.New("new client failed"),
				},
			},
			wantErr: "new client failed",
		},
		{
			name:       "new session successfully",
			sessionID:  sessionID,
			operations: &fakeClientOperation{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			session, err := newSession(c.sessionID, c.operations, config)
			if c.wantErr != "" {
				require.ErrorContains(t, err, c.wantErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, session)
				assert.Equal(t, c.sessionID, session.GetID())
				assert.Equal(t, config, session.(*sessionImpl[*fakeClient]).clientConfig)
				assert.Equal(t, c.operations, session.(*sessionImpl[*fakeClient]).ClientOperations)
				assert.Equal(t, cli, session.GetClient())
			}
		})
	}
}

func TestSessionImplCompareAndRefresh(t *testing.T) {
	const oldConfig = "{}"

	t.Run("configs are the same and no refresh", func(t *testing.T) {
		session := &sessionImpl[*fakeClient]{
			clientConfig:     oldConfig,
			ClientOperations: &fakeClientOperation{},
		}
		ok, err := session.CompareAndRefresh(oldConfig, false)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("configs are the same and force refresh", func(t *testing.T) {
		cliops := &fakeClientOperation{}
		session := &sessionImpl[*fakeClient]{
			clientConfig:     oldConfig,
			ClientOperations: cliops,
		}
		ok, err := session.CompareAndRefresh(oldConfig, true)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, 1, cliops.callTimes["Refresh"])
	})

	t.Run("configs are different and refresh failed", func(t *testing.T) {
		cliops := &fakeClientOperation{
			wantError: map[string]error{
				"Refresh": errors.New("refresh error"),
			},
		}
		session := &sessionImpl[*fakeClient]{
			clientConfig:     oldConfig,
			ClientOperations: cliops,
		}
		ok, err := session.CompareAndRefresh("", false)
		require.ErrorContains(t, err, "refresh error")
		assert.False(t, ok)
	})

	t.Run("configs are different and refresh successfully", func(t *testing.T) {
		cliops := &fakeClientOperation{}
		session := &sessionImpl[*fakeClient]{
			clientConfig:     oldConfig,
			ClientOperations: cliops,
		}
		ok, err := session.CompareAndRefresh("", false)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, 1, cliops.callTimes["Refresh"])
	})
}
