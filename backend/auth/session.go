package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Identity struct {
	Subject, Name, Email string
	Groups               []string
}

type Session struct {
	ID        string
	Identity  Identity
	ExpiresAt time.Time
	done      chan struct{}
	once      sync.Once
	timer     *time.Timer
}

func (s *Session) Done() <-chan struct{} { return s.done }

func NewUnboundedSession(identity Identity) *Session {
	return &Session{Identity: identity, done: make(chan struct{})}
}

type Sessions struct {
	mu      sync.Mutex
	entries map[string]*Session
	ttl     time.Duration
	now     func() time.Time
}

func NewSessions(ttl time.Duration) *Sessions {
	return &Sessions{entries: map[string]*Session{}, ttl: ttl, now: time.Now}
}

func (s *Sessions) Create(identity Identity) (*Session, error) {
	id, err := randomToken()
	if err != nil {
		return nil, err
	}
	session := &Session{ID: id, Identity: identity, ExpiresAt: s.now().Add(s.ttl), done: make(chan struct{})}
	s.mu.Lock()
	s.entries[id] = session
	s.mu.Unlock()
	session.timer = time.AfterFunc(time.Until(session.ExpiresAt), func() { s.expire(id, session) })
	return session, nil
}

func (s *Sessions) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.entries[id]
	if !ok {
		return nil, false
	}
	if !s.now().Before(session.ExpiresAt) {
		delete(s.entries, id)
		if session.timer != nil {
			session.timer.Stop()
		}
		session.once.Do(func() { close(session.done) })
		return nil, false
	}
	return session, true
}

func (s *Sessions) Revoke(id string) {
	s.mu.Lock()
	session := s.entries[id]
	delete(s.entries, id)
	s.mu.Unlock()
	if session != nil {
		if session.timer != nil {
			session.timer.Stop()
		}
		session.once.Do(func() { close(session.done) })
	}
}

func (s *Sessions) expire(id string, expected *Session) {
	s.mu.Lock()
	if s.entries[id] == expected {
		delete(s.entries, id)
	}
	s.mu.Unlock()
	expected.once.Do(func() { close(expected.done) })
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
