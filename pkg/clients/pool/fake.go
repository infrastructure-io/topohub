package pool

import (
	"reflect"
	"time"
)

type fakeClient struct{}

// Check implantation
var (
	_ ClientOperations[*fakeClient] = (*fakeClientOperation)(nil)
	_ Session[*fakeClient]          = (*fakeSession)(nil)
)

type fakeSession struct {
	id             string
	client         *fakeClient
	lastActiveTime time.Time

	injectPing              func() error
	injectCompareAndRefresh func(_cfg any, force bool) (bool, error)

	callTimes     map[string]int
	wantError     map[string]error
	wantRefreshed bool
}

func (s *fakeSession) init() {
	if s.callTimes == nil {
		s.callTimes = make(map[string]int)
	}
	if s.wantError == nil {
		s.wantError = make(map[string]error)
	}
}

func (s *fakeSession) GetID() string {
	return s.id
}

func (s *fakeSession) GetClient() *fakeClient {
	return s.client
}

func (s *fakeSession) Ping() error {
	s.init()
	s.callTimes["Ping"]++
	if s.injectPing != nil {
		return s.injectPing()
	}
	return s.wantError["Ping"]
}

func (s *fakeSession) CompareAndRefresh(cfg any, force bool) (bool, error) {
	s.init()
	s.callTimes["CompareAndRefresh"]++
	if s.injectCompareAndRefresh != nil {
		return s.injectCompareAndRefresh(cfg, force)
	}
	return force || s.wantRefreshed, s.wantError["CompareAndRefresh"]
}

func (s *fakeSession) Close() error {
	s.init()
	s.callTimes["Close"]++
	return s.wantError["Close"]
}

func (s *fakeSession) UpdateLastActiveTime(t time.Time) {
	s.lastActiveTime = t
}

func (s *fakeSession) GetLastActiveTime() time.Time {
	return s.lastActiveTime
}

func (s *fakeSession) LastActiveTimeIsAfterMaxIdle(maxIdle time.Duration) bool {
	return time.Since(s.lastActiveTime) > maxIdle
}

type fakeClientOperation struct {
	callTimes map[string]int
	wantError map[string]error
}

func (o *fakeClientOperation) init() {
	if o.callTimes == nil {
		o.callTimes = make(map[string]int)
	}
	if o.wantError == nil {
		o.wantError = make(map[string]error)
	}
}

func (o *fakeClientOperation) NewClient(_cfg any) (*fakeClient, error) {
	o.init()
	o.callTimes["NewClient"]++
	return &fakeClient{}, o.wantError["NewClient"]
}

func (o *fakeClientOperation) Ping(_c *fakeClient) error {
	o.init()
	o.callTimes["Ping"]++
	return o.wantError["Ping"]
}

func (o *fakeClientOperation) Compare(old, new any) bool {
	o.init()
	o.callTimes["Compare"]++
	return reflect.DeepEqual(old, new)
}

func (o *fakeClientOperation) Refresh(c *fakeClient, _cfg any) (*fakeClient, error) {
	o.init()
	o.callTimes["Refresh"]++
	return c, o.wantError["Refresh"]
}

func (o *fakeClientOperation) Close(_c *fakeClient) error {
	o.init()
	o.callTimes["Close"]++
	return o.wantError["Close"]
}
