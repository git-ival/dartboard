package actions

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestRunClusterJobsAttemptsAllAfterFailures(t *testing.T) {
	first := errors.New("first")
	last := errors.New("last")
	attempts := make([]int, 20)

	jobs := make([]int, len(attempts))
	for index := range jobs {
		jobs[index] = index
	}

	err := runClusterJobs(context.Background(), jobs, 3, strconv.Itoa, func(_ context.Context, job int) error {
		attempts[job]++
		switch job {
		case 0:
			return first
		case len(jobs) - 1:
			return last
		default:
			return nil
		}
	})
	if !errors.Is(err, first) || !errors.Is(err, last) {
		t.Fatalf("expected both failures, got %v", err)
	}

	for job, count := range attempts {
		if count != 1 {
			t.Errorf("job %d attempted %d times", job, count)
		}
	}
}

func TestRunClusterJobsBoundsConcurrency(t *testing.T) {
	var (
		active atomic.Int32
		peak   atomic.Int32
	)

	started := make(chan struct{}, 4)
	release := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- runClusterJobs(context.Background(), []int{0, 1, 2, 3}, 2, strconv.Itoa,
			func(_ context.Context, _ int) error {
				current := active.Add(1)
				for previous := peak.Load(); current > previous; previous = peak.Load() {
					if peak.CompareAndSwap(previous, current) {
						break
					}
				}

				started <- struct{}{}

				<-release
				active.Add(-1)

				return nil
			})
	}()

	<-started
	<-started
	close(release)

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if got := peak.Load(); got != 2 {
		t.Fatalf("expected two concurrent jobs, got %d", got)
	}
}

func TestRunClusterJobsStopsOnCheckpointFailure(t *testing.T) {
	var attempts int

	err := runClusterJobs(context.Background(), []int{0, 1, 2}, 1, strconv.Itoa,
		func(_ context.Context, _ int) error {
			attempts++
			return errCheckpointFailed
		})
	if !errors.Is(err, errCheckpointFailed) || attempts != 1 {
		t.Fatalf("expected one attempt and checkpoint failure, got %d, %v", attempts, err)
	}
}

func TestRunClusterJobsCancellationAndValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	run := func(context.Context, int) error {
		t.Error("unexpected job execution")
		return nil
	}
	if err := runClusterJobs(ctx, []int{0}, 1, strconv.Itoa, run); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}

	if err := runClusterJobs(context.Background(), []int{0}, 0, strconv.Itoa, run); err == nil {
		t.Fatal("expected invalid concurrency error")
	}

	if err := runClusterJobs(context.Background(), []int{}, 1, strconv.Itoa, run); err != nil {
		t.Fatalf("empty jobs: %v", err)
	}
}

func TestClusterPacerSpacesConcurrentStarts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pacer, err := newClusterPacer(5*time.Second, 0.2)
		if err != nil {
			t.Fatal(err)
		}

		starts := make(chan time.Time, 3)

		for range 3 {
			go func() {
				if err := pacer.wait(context.Background()); err != nil {
					t.Error(err)
				}

				starts <- time.Now()
			}()
		}

		previous := <-starts
		for range 2 {
			current := <-starts
			if gap := current.Sub(previous); gap < 5*time.Second || gap > 6*time.Second {
				t.Errorf("expected 5-6 second spacing, got %s", gap)
			}

			previous = current
		}
	})
}

func TestClusterPacerCancellationAndDisabledPacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pacer, err := newClusterPacer(time.Hour, 0)
		if err != nil {
			t.Fatal(err)
		}

		if err := pacer.wait(context.Background()); err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() { done <- pacer.wait(ctx) }()

		synctest.Wait()
		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}

		disabled, err := newClusterPacer(0, 0)
		if err != nil {
			t.Fatal(err)
		}

		start := time.Now()

		for range 5 {
			if err := disabled.wait(context.Background()); err != nil {
				t.Fatal(err)
			}
		}

		if time.Now() != start {
			t.Fatal("disabled pacing delayed a job")
		}
	})
}

func TestClusterPacerRejectsInvalidConfiguration(t *testing.T) {
	if _, err := newClusterPacer(-time.Second, 0); err == nil {
		t.Fatal("expected negative interval to be rejected")
	}

	if _, err := newClusterPacer(time.Second, 1.1); err == nil {
		t.Fatal("expected invalid jitter to be rejected")
	}
}
