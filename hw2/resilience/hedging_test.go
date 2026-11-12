package resilience_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Murzabek0312/highload_prog/hw2/resilience"
)

func TestHedgingZeroDelayStartsFirstReplica(t *testing.T) {
	// Given:
	// Ограничение параллелизма провоцирует запуск быстрой второй реплики раньше первой.
	previousProcs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previousProcs)
	first := newScriptedSender("first", nil, responseStep{hang: true})
	second := newScriptedSender("second", nil, responseStep{status: 200})
	strategy := resilience.Hedging{Delay: 0}

	// When:
	response, err := strategy.Do(context.Background(), []resilience.Sender{first, second}, testBudgets())

	// Then:
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if response.StatusCode != 200 {
		t.Errorf("response status = %d, want 200", response.StatusCode)
	}
	first.assertCalls(t, 1, 1)
	second.assertCalls(t, 1, 0)
}

func TestHedgingFastFirstSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		replicaA := newScriptedSender("A", nil, responseStep{status: 200, body: "first", delay: 10 * time.Millisecond})
		replicaB := newScriptedSender("B", nil, responseStep{status: 200})
		start := time.Now()
		strategy := resilience.Hedging{Delay: 20 * time.Millisecond}

		// When:
		response, err := strategy.Do(context.Background(), []resilience.Sender{replicaA, replicaB}, testBudgets())

		// Then:
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if string(response.Body) != "first" {
			t.Errorf("response body = %q, want first", response.Body)
		}
		if elapsed := time.Since(start); elapsed != 10*time.Millisecond {
			t.Errorf("elapsed = %v, want 10ms", elapsed)
		}
		replicaA.assertCalls(t, 1, 0)
		replicaB.assertCalls(t, 0, 0)
	})
}

func TestHedgingStartsRemainingReplicasTogether(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		log := &callLog{}
		replicaA := newScriptedSender("A", log, responseStep{hang: true})
		replicaB := newScriptedSender("B", log, responseStep{status: 503, delay: 5 * time.Millisecond})
		replicaC := newScriptedSender("C", log, responseStep{status: 200, body: "winner", delay: 10 * time.Millisecond})
		replicaD := newScriptedSender("D", log, responseStep{hang: true})
		limits := testBudgets()
		// Hedging должен запустить все реплики независимо от бюджетов повторов и ошибок.
		limits.MaxAttempts = 1
		limits.MaxFailures = 0
		start := time.Now()
		strategy := resilience.Hedging{Delay: 20 * time.Millisecond}

		// When:
		response, err := strategy.Do(context.Background(), []resilience.Sender{replicaA, replicaB, replicaC, replicaD}, limits)

		// Then:
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if string(response.Body) != "winner" {
			t.Errorf("response body = %q, want winner", response.Body)
		}
		if elapsed := time.Since(start); elapsed != 30*time.Millisecond {
			t.Errorf("elapsed = %v, want 30ms", elapsed)
		}
		calls := log.snapshot()
		if len(calls) != 4 || calls[0].name != "A" || calls[0].at != start {
			t.Fatalf("calls = %v", calls)
		}
		for _, call := range calls[1:] {
			if call.at.Sub(start) != 20*time.Millisecond {
				t.Fatalf("hedged call = %+v", call)
			}
		}
		replicaA.assertCalls(t, 1, 1)
		replicaB.assertCalls(t, 1, 0)
		replicaC.assertCalls(t, 1, 0)
		replicaD.assertCalls(t, 1, 1)
	})
}

func TestHedgingEarlyErrorStillLaunchesOthers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		log := &callLog{}
		replicaA := newScriptedSender("A", log, responseStep{status: 400})
		replicaB := newScriptedSender("B", log, responseStep{status: 200, body: "ok"})
		start := time.Now()
		strategy := resilience.Hedging{Delay: 20 * time.Millisecond}

		// When:
		response, err := strategy.Do(context.Background(), []resilience.Sender{replicaA, replicaB}, testBudgets())

		// Then:
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if string(response.Body) != "ok" {
			t.Errorf("unexpected response: %+v", response)
		}
		assertCallSequence(t, log, start, []expectedCall{
			{name: "A"},
			{name: "B", after: 20 * time.Millisecond},
		})
		replicaA.assertCalls(t, 1, 0)
		replicaB.assertCalls(t, 1, 0)
	})
}

func TestHedgingAllFailuresWaitForDeadline(t *testing.T) {
	tests := []struct {
		name           string
		firstResponse  responseStep
		secondResponse responseStep
		wantCanceled   int
	}{
		{
			name:           "errors",
			firstResponse:  responseStep{status: 400},
			secondResponse: responseStep{status: 500},
		},
		{
			name:           "timeouts",
			firstResponse:  responseStep{hang: true},
			secondResponse: responseStep{hang: true},
			wantCanceled:   1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Given:
				replicaA := newScriptedSender("A", nil, test.firstResponse)
				replicaB := newScriptedSender("B", nil, test.secondResponse)
				strategy := resilience.Hedging{Delay: 20 * time.Millisecond}
				start := time.Now()

				// When:
				_, err := strategy.Do(context.Background(), []resilience.Sender{replicaA, replicaB}, testBudgets())

				// Then:
				if !errors.Is(err, resilience.ErrLatencyExhausted) {
					t.Fatalf("Do() error = %v, want latency exhaustion", err)
				}
				if elapsed := time.Since(start); elapsed != time.Second {
					t.Errorf("elapsed = %v, want %v", elapsed, time.Second)
				}
				replicaA.assertCalls(t, 1, test.wantCanceled)
				replicaB.assertCalls(t, 1, test.wantCanceled)
			})
		})
	}
}

func TestHedgingCallerCancellationAndShortDeadline(t *testing.T) {
	tests := []struct {
		name            string
		latency         time.Duration
		cancelAfter     time.Duration
		wantSecondCalls int
		wantErr         error
	}{
		{
			name:    "deadline before hedge",
			latency: 10 * time.Millisecond,
			wantErr: resilience.ErrLatencyExhausted,
		},
		{
			name:            "cancel active hedges",
			latency:         time.Second,
			cancelAfter:     30 * time.Millisecond,
			wantSecondCalls: 1,
			wantErr:         context.Canceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Given:
				limits := testBudgets()
				limits.Latency = test.latency
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if test.cancelAfter > 0 {
					go func() {
						time.Sleep(test.cancelAfter)
						cancel()
					}()
				}
				replicaA := newScriptedSender("A", nil, responseStep{hang: true})
				replicaB := newScriptedSender("B", nil, responseStep{hang: true})
				strategy := resilience.Hedging{Delay: 20 * time.Millisecond}

				// When:
				_, err := strategy.Do(ctx, []resilience.Sender{replicaA, replicaB}, limits)

				// Then:
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Do() error = %v, want %v", err, test.wantErr)
				}
				replicaA.assertCalls(t, 1, 1)
				replicaB.assertCalls(t, test.wantSecondCalls, test.wantSecondCalls)
			})
		})
	}
}
