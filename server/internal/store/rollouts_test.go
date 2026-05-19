package store

import (
	"sync"
	"sync/atomic"
	"testing"
)

/*
TestClaimRollout_ExactlyOneWinnerUnderRace guards the audit fix:
two concurrent boots both calling ClaimRollout on the same slug must
have exactly one return (true, nil). The previous read-then-write
pattern (IsRolloutSent → SendTradeEmail → MarkRolloutSent) let both
processes observe sent=false and both bulk-email every subscriber.
*/
func TestClaimRollout_ExactlyOneWinnerUnderRace(t *testing.T) {
	s := setupTestDB(t)

	const goroutines = 16
	var wg sync.WaitGroup
	var wins int32

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			won, err := s.ClaimRollout("test-rollout-race", 42)
			if err != nil {
				t.Errorf("ClaimRollout: %v", err)
				return
			}
			if won {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&wins); got != 1 {
		t.Fatalf("expected exactly 1 winner under race, got %d", got)
	}

	// IsRolloutSent should now return true.
	sent, err := s.IsRolloutSent("test-rollout-race")
	if err != nil {
		t.Fatalf("IsRolloutSent: %v", err)
	}
	if !sent {
		t.Fatal("post-claim IsRolloutSent should report sent=true")
	}
}

/*
TestClaimRollout_SecondAttemptReturnsFalse covers the simple
sequential case: a second call for an already-claimed slug returns
(false, nil) without error.
*/
func TestClaimRollout_SecondAttemptReturnsFalse(t *testing.T) {
	s := setupTestDB(t)

	won, err := s.ClaimRollout("test-rollout-seq", 5)
	if err != nil || !won {
		t.Fatalf("first claim should succeed, got won=%v err=%v", won, err)
	}

	won, err = s.ClaimRollout("test-rollout-seq", 5)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if won {
		t.Fatal("second claim should return won=false")
	}
}
