package inmemory

import (
	"context"
	"errors"
	"sync"

	le "leaderelection"
)

type LeaseStore struct {
	mu    sync.RWMutex
	lease *le.Lease
}

// NewInMemoryLeaseStore creates a new InMemoryLeaseStore.
func NewInMemoryLeaseStore() *LeaseStore {
	return &LeaseStore{}
}

// GetLease retrieves the current lease.
func (s *LeaseStore) GetLease(_ context.Context) (*le.Lease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lease == nil {
		return nil, le.ErrLeaseNotFound
	}
	return s.lease, nil
}

// CreateLease creates a new lease.
func (s *LeaseStore) CreateLease(_ context.Context, lease *le.Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lease != nil {
		return errors.New("lease already exists")
	}
	s.lease = lease
	return nil
}

// UpdateLease updates the existing lease.
func (s *LeaseStore) UpdateLease(_ context.Context, lease *le.Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lease == nil {
		return errors.New("lease not found")
	}
	s.lease = lease
	return nil
}
