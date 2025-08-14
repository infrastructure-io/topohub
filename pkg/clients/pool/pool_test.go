package pool

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/infrastructure-io/topohub/pkg/log"
)

func TestSessionPoolImplGetSession(t *testing.T) {
	const sessionID = "1000"
	pool := &sessionPoolImpl[*fakeClient]{
		cache: make(map[string]Session[*fakeClient]),
		log:   log.Logger,
	}

	t.Run("no session found", func(t *testing.T) {
		session, err := pool.getSession(sessionID, nil)
		require.NoError(t, err)
		assert.Nil(t, session)
	})

	t.Run("compare and refresh session failed", func(t *testing.T) {
		pool.cache[sessionID] = &fakeSession{
			wantError: map[string]error{
				"CompareAndRefresh": fmt.Errorf("test error"),
			},
		}
		session, err := pool.getSession(sessionID, nil)
		require.ErrorContains(t, err, "refresh session failed when config changed")
		assert.Nil(t, session)
	})

	t.Run("ping session failed and has refreshed", func(t *testing.T) {
		pool.cache[sessionID] = &fakeSession{
			wantRefreshed: true,
			wantError: map[string]error{
				"Ping": fmt.Errorf("test error"),
			},
		}
		session, err := pool.getSession(sessionID, nil)
		require.ErrorIs(t, err, ErrSessionPingFailed)
		assert.Nil(t, session)
	})

	t.Run("ping session failed and refresh failed", func(t *testing.T) {
		pool.cache[sessionID] = &fakeSession{
			injectCompareAndRefresh: func(_cfg any, force bool) (bool, error) {
				if force {
					return false, errors.New("force refresh error")
				}
				return false, nil
			},
			wantError: map[string]error{
				"Ping": fmt.Errorf("test error"),
			},
		}
		session, err := pool.getSession(sessionID, nil)
		require.ErrorContains(t, err, "refresh session failed when ping failed")
		assert.Nil(t, session)
	})

	t.Run("ping session failed and refresh successfully and ping session failed again", func(t *testing.T) {
		fake := &fakeSession{
			wantError: map[string]error{
				"Ping": fmt.Errorf("test error"),
			},
		}
		pool.cache[sessionID] = fake
		_, err := pool.getSession(sessionID, nil)
		require.ErrorIs(t, err, ErrSessionPingFailed)
		assert.Equal(t, 2, fake.callTimes["Ping"])
		assert.Equal(t, 2, fake.callTimes["CompareAndRefresh"])
	})

	t.Run("ping session failed and refresh successfully and ping session successfully", func(t *testing.T) {
		var pingTimes int
		fake := &fakeSession{
			injectPing: func() error {
				if pingTimes == 0 {
					pingTimes++
					return fmt.Errorf("test error")
				}
				return nil
			},
		}
		pool.cache[sessionID] = fake
		session, err := pool.getSession(sessionID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, 2, fake.callTimes["Ping"])
		assert.Equal(t, 2, fake.callTimes["CompareAndRefresh"])
	})

	t.Run("get session successfully", func(t *testing.T) {
		fake := &fakeSession{}
		pool.cache[sessionID] = fake
		session, err := pool.getSession(sessionID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, 1, fake.callTimes["Ping"])
		assert.Equal(t, 1, fake.callTimes["CompareAndRefresh"])
	})
}

func TestSessionPoolImplCreateSession(t *testing.T) {
	const sessionID = "1000"
	pool := &sessionPoolImpl[*fakeClient]{
		cache: make(map[string]Session[*fakeClient]),
		log:   log.Logger,
	}

	t.Run("create session failed", func(t *testing.T) {
		clear(pool.cache)
		pool.cliops = &fakeClientOperation{
			wantError: map[string]error{
				"NewClient": fmt.Errorf("test error"),
			},
		}
		_, err := pool.createSession(sessionID, nil)
		require.ErrorContains(t, err, "create session failed")
	})

	t.Run("ping session failed during creating", func(t *testing.T) {
		clear(pool.cache)
		pool.cliops = &fakeClientOperation{
			wantError: map[string]error{
				"Ping": fmt.Errorf("test error"),
			},
		}
		_, err := pool.createSession(sessionID, nil)
		require.ErrorIs(t, err, ErrSessionPingFailed)
	})

	t.Run("create session successfully", func(t *testing.T) {
		clear(pool.cache)
		fake := &fakeClientOperation{}
		pool.cliops = fake

		session, err := pool.createSession(sessionID, nil)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, sessionID, session.GetID())
		assert.Equal(t, 1, fake.callTimes["Ping"])
		assert.Equal(t, 1, fake.callTimes["NewClient"])
		assert.True(t, session.GetLastActiveTime().Unix() > 0)
	})

	t.Run("test concurrent call", func(t *testing.T) {
		clear(pool.cache)
		fake := &fakeClientOperation{}
		pool.cliops = fake

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			session, err := pool.createSession(sessionID, nil)
			require.NoError(t, err)
			require.NotNil(t, session)
			assert.True(t, session.GetLastActiveTime().Unix() > 0)
		}()

		go func() {
			defer wg.Done()
			session, err := pool.createSession(sessionID, nil)
			require.NoError(t, err)
			require.NotNil(t, session)
			assert.True(t, session.GetLastActiveTime().Unix() > 0)
		}()

		wg.Wait()
		assert.Equal(t, 2, fake.callTimes["Ping"])
		assert.Equal(t, 1, fake.callTimes["NewClient"])
	})
}

func TestSessionPoolImplRefresh(t *testing.T) {
	const sessionID = "1000"
	pool := &sessionPoolImpl[*fakeClient]{
		cache: make(map[string]Session[*fakeClient]),
		log:   log.Logger,
	}

	t.Run("compare and refresh session failed", func(t *testing.T) {
		pool.cache[sessionID] = &fakeSession{
			wantError: map[string]error{
				"CompareAndRefresh": fmt.Errorf("test error"),
			},
		}
		_, err := pool.Refresh(sessionID, nil)
		require.ErrorContains(t, err, "refresh session failed")
	})

	t.Run("ping session failed", func(t *testing.T) {
		pool.cache[sessionID] = &fakeSession{
			wantRefreshed: true,
			wantError: map[string]error{
				"Ping": fmt.Errorf("test error"),
			},
		}
		_, err := pool.Refresh(sessionID, nil)
		require.ErrorIs(t, err, ErrSessionPingFailed)
	})

	t.Run("refresh session successfully", func(t *testing.T) {
		pool.cache[sessionID] = &fakeSession{
			wantRefreshed: true,
		}
		session, err := pool.Refresh(sessionID, nil)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.True(t, session.GetLastActiveTime().Unix() > 0)
	})
}

func TestSessionPoolImplClean(t *testing.T) {
	session001 := &fakeSession{
		lastActiveTime: time.Now().Add(-2 * time.Hour),
	}
	pool := &sessionPoolImpl[*fakeClient]{
		maxIdleHour: 1,
		cache: map[string]Session[*fakeClient]{
			"1000": session001,
			"1001": &fakeSession{
				lastActiveTime: time.Now().Add(-50 * time.Minute),
			},
		},
		log: log.Logger,
	}
	pool.cleanup()
	assert.Len(t, pool.cache, 1)
	assert.NotNil(t, pool.cache["1001"])
	assert.Equal(t, 1, session001.callTimes["Close"])
}
