package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrSessionNotFound = errors.New("session not found")

	ErrSessionPingFailed = errors.New("session ping failed")
)

type SessionPool[T any] interface {
	// GetOrCreate get the session from cache, if not found, create a new one.
	GetOrCreate(sessionID string, cfg any) (Session[T], error)

	// Refresh refresh the session config.
	Refresh(sessionID string, cfg any) (Session[T], error)
}

// NewSessionFunc define the function to create a new session.
type NewSessionFunc[T any] func(id string, cfg any) (Session[T], error)

// NewSessionPool create a new session pool.
func NewSessionPool[T any](ctx context.Context, cliops ClientOperations[T], opts ...Option) SessionPool[T] {
	options := buildOptions(opts...)
	pool := &sessionPoolImpl[T]{
		maxIdleHour:    options.maxIdleHour,
		gcIntervalHour: options.gcIntervalHour,
		cliops:         cliops,
		cache:          make(map[string]Session[T]),
		log:            options.logger,
	}
	go pool.gc(ctx)
	return pool
}

// Check implantation
var _ SessionPool[any] = (*sessionPoolImpl[any])(nil)

type sessionPoolImpl[T any] struct {
	maxIdleHour    int32
	gcIntervalHour int32
	cliops         ClientOperations[T]
	cache          map[string]Session[T]
	log            *zap.SugaredLogger
	sync.RWMutex
}

func (c *sessionPoolImpl[T]) GetOrCreate(sessionID string, cfg any) (Session[T], error) {
	session, err := c.getSession(sessionID, cfg)
	if err != nil {
		return nil, err
	}
	if session != nil {
		return session, nil
	}

	// No session found, create a new session
	session, err = c.createSession(sessionID, cfg)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (c *sessionPoolImpl[T]) getSession(sessionID string, cfg any) (Session[T], error) {
	c.RLock()
	defer c.RUnlock()

	session, ok := c.cache[sessionID]
	if !ok || session == nil {
		return nil, nil
	}
	refreshed, err := session.CompareAndRefresh(cfg, false)
	if err != nil {
		return nil, fmt.Errorf("refresh session failed when config changed, err: %v", err)
	}
	if refreshed {
		c.log.Infof("the inported session %s config is different, refresh the session", sessionID)
	}
	if err := session.Ping(); err != nil {
		if !refreshed {
			c.log.Infof("ping found session %s failed, refresh the session", sessionID)
			if _, err := session.CompareAndRefresh(cfg, true); err != nil {
				return nil, fmt.Errorf("refresh session failed when ping failed, err: %v", err)
			}
			if err := session.Ping(); err != nil {
				return nil, ErrSessionPingFailed
			}
			return session, nil
		}
		return nil, ErrSessionPingFailed
	}
	return session, nil
}

func (c *sessionPoolImpl[T]) createSession(sessionID string, cfg any) (Session[T], error) {
	c.Lock()
	defer c.Unlock()

	// Double check
	if session, ok := c.cache[sessionID]; ok {
		if err := session.Ping(); err != nil {
			return nil, ErrSessionPingFailed
		}
		return session, nil
	}

	session, err := newSession(sessionID, c.cliops, cfg)
	if err != nil {
		return nil, fmt.Errorf("create session failed, err: %v", err)
	}
	if err := session.Ping(); err != nil {
		return nil, ErrSessionPingFailed
	}
	session.UpdateLastActiveTime(time.Now())
	c.cache[sessionID] = session
	return session, nil
}

func (c *sessionPoolImpl[T]) Refresh(sessionID string, cfg any) (Session[T], error) {
	c.Lock()
	defer c.Unlock()

	session := c.cache[sessionID]
	if session == nil {
		return nil, ErrSessionNotFound
	}

	refreshed, err := session.CompareAndRefresh(cfg, false)
	if err != nil {
		return nil, fmt.Errorf("refresh session failed, err: %v", err)
	}
	if refreshed {
		c.log.Infof("the session %s config is not changed, skip refresh", sessionID)
	}
	if err := session.Ping(); err != nil {
		return nil, ErrSessionPingFailed
	}
	session.UpdateLastActiveTime(time.Now())
	return session, nil
}

func (c *sessionPoolImpl[T]) gc(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.gcIntervalHour) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-ctx.Done():
			return
		}
	}
}

// cleanup clean up expired sessions.
func (c *sessionPoolImpl[T]) cleanup() {
	for sessionID := range c.cache {
		session := c.cache[sessionID]
		if session == nil {
			continue
		}
		if session.LastActiveTimeIsAfterMaxIdle(time.Duration(c.maxIdleHour) * time.Hour) {
			c.Lock()
			delete(c.cache, sessionID)
			if err := session.Close(); err != nil {
				c.log.Warnf("close session %s failed, err: %v", sessionID, err)
			}
			c.Unlock()
		}
	}
}
