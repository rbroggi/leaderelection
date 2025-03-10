package leaderelection

import (
	"context"
	"errors"
	"sync"
	"time"
)

const errLogKey = "error"

var log Logger = defaultLog

func SetLogger(lgr Logger) {
	log = lgr
}

// ElectorConfig holds the configuration for the Elector.
type ElectorConfig struct {
	LeaseDuration    time.Duration
	RetryPeriod      time.Duration
	LeaseStore       LeaseStore
	CandidateID      string
	OnStartedLeading func(ctx context.Context)
	OnStoppedLeading func()
	OnNewLeader      func(identity string)
	ReleaseOnCancel  bool // Add this flag.
}

// Elector performs leader election.
type Elector struct {
	config ElectorConfig
	// internal bookkeeping
	observedRecord     Lease
	observedTime       time.Time
	observedRecordLock sync.RWMutex
}

func NewElector(cfg ElectorConfig) (*Elector, error) {
	if cfg.LeaseDuration < 1 {
		return nil, errors.New("lease duration must be greater than zero")
	}
	if cfg.RetryPeriod < 1 {
		return nil, errors.New("retry period must be greater than zero")
	}
	if cfg.OnStartedLeading == nil {
		return nil, errors.New("OnStartedLeading callback is required")
	}
	if cfg.OnStoppedLeading == nil {
		return nil, errors.New("OnStoppedLeading callback is required")
	}
	if cfg.OnNewLeader == nil {
		return nil, errors.New("OnNewLeader callback is required")
	}

	return &Elector{
		config: cfg,
	}, nil
}

// Run starts the leader election process, it is a non-blocking call and returns a channel to signal completion.
func (le *Elector) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range time.Tick(le.config.RetryPeriod) {
			select {
			case <-ctx.Done():
				log.Info("Context cancelled, stopping leader election.", "candidate", le.config.CandidateID)
				le.release()
				return
			default:
				if acquired := le.tryAcquireOrRenew(ctx); acquired {
					log.Debug("Acquired or renewed lease", "candidate", le.config.CandidateID, "lease", le.getObservedRecord())
				}
			}
		}
	}()
	return done
}

func (le *Elector) tryAcquireOrRenew(ctx context.Context) bool {
	now := time.Now()
	leaseLeaderRecord := &Lease{
		HolderIdentity: le.config.CandidateID,
		LeaseDuration:  le.config.LeaseDuration,
		RenewTime:      now,
		AcquireTime:    now,
	}

	lease, err := le.config.LeaseStore.GetLease(ctx)
	if err != nil {
		if !errors.Is(err, ErrLeaseNotFound) {
			log.Error("Error getting lease", errLogKey, err)
			return false
		}
		if err := le.config.LeaseStore.CreateLease(ctx, leaseLeaderRecord); err != nil {
			log.Error("Error creating lease", errLogKey, err)
			return false
		}
		le.setObservedRecord(leaseLeaderRecord)
		le.config.OnStartedLeading(ctx)
		return true
	}

	log.Debug("try acquiring/renewing lease", "candidate", le.config.CandidateID, "lease", lease)

	if !le.equalLastObservedRecord(lease) {
		if le.getObservedRecord().HolderIdentity != lease.HolderIdentity {
			le.config.OnNewLeader(lease.HolderIdentity)
		}
		le.setObservedRecord(lease)
	}

	if le.heldByOtherCandidateAndNotExpired(lease) {
		log.Debug("lease held by other candidate not yet expired", "candidate", le.config.CandidateID)
		return false
	}

	var takeover bool
	if le.IsLeader() {
		leaseLeaderRecord.AcquireTime = lease.AcquireTime
		leaseLeaderRecord.LeaderTransitions = lease.LeaderTransitions
	} else {
		leaseLeaderRecord.LeaderTransitions = lease.LeaderTransitions + 1
		takeover = true
	}

	if err := le.config.LeaseStore.UpdateLease(ctx, leaseLeaderRecord); err != nil {
		log.Error("Failed to update lease", errLogKey, err)
		return false
	}

	le.setObservedRecord(leaseLeaderRecord)
	if takeover {
		le.config.OnStartedLeading(ctx)
	}

	return true
}

func expired(
	lastObservedTime, now time.Time,
	leaseDuration time.Duration,
) bool {
	return lastObservedTime.Add(leaseDuration).Before(now)
}

func (le *Elector) release() bool {
	if !le.IsLeader() {
		return true
	}

	le.config.OnStoppedLeading()

	now := time.Now()
	releasedLease := &Lease{
		LeaderTransitions: le.observedRecord.LeaderTransitions,
		// Reset the holder identity to release the lease.
		HolderIdentity: "",
		LeaseDuration:  le.config.LeaseDuration,
		RenewTime:      now,
		AcquireTime:    now,
	}

	le.setObservedRecord(releasedLease)

	if le.config.ReleaseOnCancel {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := le.config.LeaseStore.UpdateLease(ctx, releasedLease); err != nil {
			log.Error("Failed to release lease", errLogKey, err)
			return false
		}
	}

	return true
}

func (le *Elector) IsLeader() bool {
	return le.getObservedRecord().HolderIdentity == le.config.CandidateID
}

func (le *Elector) setObservedRecord(lease *Lease) {
	le.observedRecordLock.Lock()
	defer le.observedRecordLock.Unlock()
	le.observedRecord = *lease
	le.observedTime = time.Now()
}

// getObservedRecord returns observersRecord.
// Protect critical sections with lock.
func (le *Elector) getObservedRecord() Lease {
	le.observedRecordLock.RLock()
	defer le.observedRecordLock.RUnlock()
	return le.observedRecord
}

func (le *Elector) equalLastObservedRecord(lease *Lease) bool {
	le.observedRecordLock.RLock()
	defer le.observedRecordLock.RUnlock()
	return le.observedRecord.HolderIdentity == lease.HolderIdentity &&
		le.observedRecord.AcquireTime.Equal(lease.AcquireTime) &&
		le.observedRecord.RenewTime.Equal(lease.RenewTime) &&
		le.observedRecord.LeaseDuration == lease.LeaseDuration &&
		le.observedRecord.LeaderTransitions == lease.LeaderTransitions
}

func (le *Elector) heldByOtherCandidateAndNotExpired(lease *Lease) bool {
	return lease.HasHolder() && !le.IsLeader() && !expired(le.observedTime, time.Now(), lease.LeaseDuration)
}
