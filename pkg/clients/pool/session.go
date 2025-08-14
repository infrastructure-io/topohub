package pool

import (
	"fmt"
	"sync"
	"time"
)

// Session is a item in the pool.
type Session[T any] interface {
	// GetID returns the session ID.
	GetID() string

	// GetClient returns the client.
	GetClient() T

	// Ping pings the session.
	Ping() error

	// CompareAndRefresh refreshes the session with the given config.
	// If the config has not changed and force is false, no refresh will be performed.
	// Returns whether the session has been refreshed.
	// Usually after the authentication information or connection information changes,
	// this method is used to re-establish the connection.
	CompareAndRefresh(cfg any, force bool) (bool, error)

	// Close closes the session.
	Close() error

	// UpdateLastActiveTime updates the last active time.
	UpdateLastActiveTime(t time.Time)

	// GetLastActiveTime returns the last active time.
	GetLastActiveTime() time.Time

	// LastActiveTimeIsAfterMaxIdle returns true if the last active time is after max idle time.
	LastActiveTimeIsAfterMaxIdle(maxIdle time.Duration) bool
}

type ClientOperations[T any] interface {
	// NewClient creates a new client.
	NewClient(cfg any) (T, error)

	// Ping pings the client.
	Ping(client T) error

	// Compare compares the old and new client configurations.
	// Returns true if they are the same.
	Compare(old, new any) bool

	// Refresh refreshes the client with the given config, and returns a new client.
	Refresh(oldClient T, cfg any) (T, error)

	// Close closes the client.
	Close(client T) error
}

// newSession creates a new session.
func newSession[T any](sessionID string, operations ClientOperations[T], cfg any) (Session[T], error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session id must not be empty")
	}
	if operations == nil {
		return nil, fmt.Errorf("operations must not be nil")
	}
	client, err := operations.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &sessionImpl[T]{
		id:               sessionID,
		clientConfig:     cfg,
		client:           client,
		ClientOperations: operations,
	}, nil
}

// Check implantation
var _ Session[any] = (*sessionImpl[any])(nil)

type sessionImpl[T any] struct {
	// id of the session.
	id string

	// cfg is a configuration for creating the client
	clientConfig any

	// client represents the actual client, assigned by the NewClient function.
	client T

	// lastActiveTime is used for the pool to check if the session is expired.
	// If the session is not active for a long time, it will be closed and removed from the pool.
	lastActiveTime time.Time

	// ClientOperations provides client-side functions for session operations.
	ClientOperations[T]

	// Session-level locks
	sync.RWMutex
}

func (s *sessionImpl[T]) GetID() string {
	return s.id
}

func (s *sessionImpl[T]) GetClient() T {
	s.RLock()
	defer s.RUnlock()

	return s.client
}

// Ping pings the client.
func (s *sessionImpl[T]) Ping() error {
	return s.ClientOperations.Ping(s.client)
}

func (s *sessionImpl[T]) CompareAndRefresh(cfg any, force bool) (bool, error) {
	s.Lock()
	defer s.Unlock()

	if !force && s.Compare(s.clientConfig, cfg) {
		return false, nil
	}
	client, err := s.Refresh(s.client, cfg)
	if err != nil {
		return false, err
	}
	s.client = client
	s.clientConfig = cfg
	return true, nil
}

func (s *sessionImpl[T]) Close() error {
	s.Lock()
	defer s.Unlock()
	return s.ClientOperations.Close(s.client)
}

func (s *sessionImpl[T]) UpdateLastActiveTime(t time.Time) {
	s.Lock()
	defer s.Unlock()
	s.lastActiveTime = t
}

func (s *sessionImpl[T]) LastActiveTimeIsAfterMaxIdle(maxIdle time.Duration) bool {
	s.RLock()
	defer s.RUnlock()
	return time.Since(s.lastActiveTime) > maxIdle
}

func (s *sessionImpl[T]) GetLastActiveTime() time.Time {
	s.RLock()
	defer s.RUnlock()
	return s.lastActiveTime
}
