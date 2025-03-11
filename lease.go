package leaderelection

import (
	"time"
)

// Lease represents a lease for leadership.
type Lease struct {
	HolderIdentity    string
	AcquireTime       time.Time
	RenewTime         time.Time
	LeaseDuration     time.Duration
	LeaderTransitions uint32
}

func (l *Lease) HasHolder() bool {
	return l != nil && l.HolderIdentity != ""
}
