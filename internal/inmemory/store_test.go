package inmemory

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	le "github.com/rbroggi/leaderelection"
)

func TestLeaderElection(t *testing.T) {
	t.Parallel()

	// --- Test Configuration ---
	leaseDuration := 1 * time.Second
	retryPeriod := 200 * time.Millisecond
	candidates := []string{
		"candidate-1",
		"candidate-2",
		"candidate-3",
	}

	store := NewInMemoryLeaseStore() // Use the *same* key.
	// Ensure that there is no existing lease

	// --- Test Context ---
	electors := make(map[string]leaderAndCnl) // Keep track of electors for later shutdown
	// --- Scenario 1: No Leader, First Leader Election ---
	t.Run("Initial Election", func(t *testing.T) {
		for _, candidateID := range candidates {
			config := le.ElectorConfig{
				LeaseDuration:   leaseDuration,
				RetryPeriod:     retryPeriod,
				LeaseStore:      store,
				CandidateID:     candidateID,
				ReleaseOnCancel: true, // Important for later scenarios.
				OnStartedLeading: func(ctx context.Context) {
				},
				OnStoppedLeading: func() {
				},
				OnNewLeader: func(identity string) {
				},
			}
			elector, err := le.NewElector(config)
			require.NoError(t, err, "Failed to create elector")
			ctx, cancel := context.WithCancel(context.Background())
			ch := elector.Run(ctx)
			electors[candidateID] = leaderAndCnl{
				cancel:  cancel,
				elector: elector,
				done:    ch,
			}
		}

		require.Eventually(t, func() bool {
			leadersCount := countLeaders(electors)
			return leadersCount == 1
		}, 2*retryPeriod, 100*time.Millisecond, "There should be exactly one leader [%d] found", countLeaders(electors))

		require.Neverf(t, func() bool {
			leadersCount := countLeaders(electors)
			return leadersCount != 1
		}, 2*retryPeriod, 100*time.Millisecond, "There should be exactly one leader")
	})

	// --- Scenario 2: Graceful Shutdown and Leader Takeover ---
	t.Run("Graceful Shutdown and Takeover", func(t *testing.T) {
		// Find current leader
		currentLeader := findLeader(electors)
		require.NotEmpty(t, currentLeader, "There must be a leader for this test")

		log.Printf("Current leader before shutdown: %s", currentLeader)

		// Simulate graceful shutdown of current leader.
		// find index of the current leader elector
		electors[currentLeader].cancel() // Stop the elector for the current leader.

		// Eventually elector stops (done)
		assert.Eventually(t, func() bool {
			// Check if the elector is done
			select {
			case <-electors[currentLeader].done:
				return true
			default:
				return false
			}
		}, 2*retryPeriod, 100*time.Millisecond)

		// Check New Leader
		assert.Eventually(t, func() bool {
			c := countLeaders(electors)
			newLeader := findLeader(electors)
			return newLeader != "" && newLeader != currentLeader && c == 1
		}, 2*leaseDuration, 100*time.Millisecond,
			"A single new leader should be elected after shutdown. Old leader %s, new leader %s [count: %d]",
			currentLeader, findLeader(electors), countLeaders(electors))
	})

	// cancel all electors to clean up
	for _, elector := range electors {
		elector.cancel()
	}
	// Eventually all electors stop (done)
	assert.Eventually(t, func() bool {
		// Check if the elector is done
		for _, elector := range electors {
			select {
			case <-elector.done:
				continue
			default:
				return false
			}
		}
		return true
	}, 2*retryPeriod, 100*time.Millisecond)

	// Eventually no leader
	assert.Eventually(t, func() bool {
		newLeader := findLeader(electors)
		return newLeader == ""
	}, 4*time.Second, 100*time.Millisecond,
		"All leaders should be stopped and there should be no leader after shutdown")

	//--- Scenario 3: Lease Expiration and Takeover ---
	t.Run("Lease Expiration and Takeover", func(t *testing.T) {
		for _, candidateID := range candidates {
			config := le.ElectorConfig{
				LeaseDuration:   leaseDuration,
				RetryPeriod:     retryPeriod,
				LeaseStore:      store,
				CandidateID:     candidateID,
				ReleaseOnCancel: false,
				OnStartedLeading: func(ctx context.Context) {
				},
				OnStoppedLeading: func() {
				},
				OnNewLeader: func(identity string) {
				},
			}

			elector, err := le.NewElector(config)
			require.NoError(t, err, "Failed to create elector")
			ctx, cancel := context.WithCancel(context.Background())
			ch := elector.Run(ctx)
			electors[candidateID] = leaderAndCnl{
				cancel:  cancel,
				elector: elector,
				done:    ch,
			}
		}

		require.Eventually(t, func() bool {
			leadersCount := countLeaders(electors)
			return leadersCount == 1
		}, 3*retryPeriod, 100*time.Millisecond, "There should be exactly one leader [%d found]", countLeaders(electors))

		// Find Current Leader
		currentLeader := findLeader(electors)
		require.NotEmpty(t, currentLeader, "There must be a leader for this test")
		log.Printf("Current leader before lease expiration: %s", currentLeader)
		// cancel leader
		electors[currentLeader].cancel() // Stop the elector for the current leader.

		// Wait for the elector to stop
		// Eventually elector stops (done)
		assert.Eventually(t, func() bool {
			// Check if the elector is done
			select {
			case <-electors[currentLeader].done:
				return true
			default:
				return false
			}
		}, 2*retryPeriod, 100*time.Millisecond)

		assert.Eventually(t, func() bool {
			newLeader := findLeader(electors)
			return newLeader != "" && newLeader != currentLeader
		}, 2*leaseDuration, 100*time.Millisecond, "A new leader should be elected after shutdown")

		// cancel all electors to clean up
		for _, elector := range electors {
			elector.cancel()
		}
		// Wait for all electors to stop
		// Eventually all electors stop (done)
		assert.Eventually(t, func() bool {
			// Check if the elector is done
			for _, elector := range electors {
				select {
				case <-elector.done:
					continue
				default:
					return false
				}
			}
			return true
		}, 2*retryPeriod, 100*time.Millisecond)

		// Eventually no leader
		assert.Eventually(t, func() bool {
			newLeader := findLeader(electors)
			return newLeader == ""
		}, 2*leaseDuration, 100*time.Millisecond, "A new leader should be elected after shutdown")
	})
}

type leaderAndCnl struct {
	cancel  context.CancelFunc
	done    <-chan struct{}
	elector *le.Elector
}

func countLeaders(electors map[string]leaderAndCnl) int {
	// count electors that are leading
	leadersCount := 0
	for _, elector := range electors {
		if elector.elector.IsLeader() {
			leadersCount++
		}
	}
	return leadersCount
}

func findLeader(electors map[string]leaderAndCnl) string {
	for candidate, elector := range electors {
		if elector.elector.IsLeader() {
			return candidate
		}
	}
	return ""
}
