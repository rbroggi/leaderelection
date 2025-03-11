package leaderelection

import (
	"context"
	"errors"
)

var (
	// ErrLeaseNotFound is returned when the lease is not found.
	ErrLeaseNotFound = errors.New("lease not found")
)

type LeaseStore interface {
	// GetLease retrieves the current lease. Should return ErrLeaseNotFound if the
	// lease does not exist.
	GetLease(ctx context.Context) (*Lease, error)
	// UpdateLease updates the lease if the lease exists.
	UpdateLease(ctx context.Context, newLease *Lease) error
	// CreateLease creates a new lease if one does not exist.
	CreateLease(ctx context.Context, newLease *Lease) error
}
