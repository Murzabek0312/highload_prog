package resilience_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Murzabek0312/highload_prog/hw2/resilience"
)

func TestRoundRobinOrderAndBackoffComposition(t *testing.T) {
	tests := []struct {
		name      string
		strategy  resilience.Client
		wantCalls []expectedCall
	}{
		{
			name: "retry",
			wantCalls: []expectedCall{
				{name: "A"},
				{name: "B"},
				{name: "C"},
				{name: "A"},
			},
		},
		{
			name:     "backoff",
			strategy: resilience.Backoff{InitialDelay: 10 * time.Millisecond, Multiplier: 2},
			wantCalls: []expectedCall{
				{name: "A"},
				{name: "B", after: 10 * time.Millisecond},
				{name: "C", after: 10 * time.Millisecond},
				{name: "A", after: 30 * time.Millisecond},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Given:
				log := &callLog{}
				replicaA := newScriptedSender("A", log,
					responseStep{status: 500},
					responseStep{status: 200, body: "winner"},
				)
				replicaB := newScriptedSender("B", log, responseStep{status: 400})
				replicaC := newScriptedSender("C", log, responseStep{status: 503})
				senders := []resilience.Sender{replicaA, replicaB, replicaC}
				strategy := resilience.RoundRobin{Strategy: test.strategy}
				start := time.Now()

				// When:
				response, err := strategy.Do(context.Background(), senders, testBudgets())

				// Then:
				if err != nil {
					t.Fatalf("Do() error = %v", err)
				}
				if string(response.Body) != "winner" {
					t.Errorf("response body = %q, want winner", response.Body)
				}
				assertCallSequence(t, log, start, test.wantCalls)
				replicaA.assertCalls(t, 2, 0)
				replicaB.assertCalls(t, 1, 0)
				replicaC.assertCalls(t, 1, 0)
			})
		})
	}
}

func TestRoundRobinBudgetIsShared(t *testing.T) {
	// Given:
	limits := testBudgets()
	limits.MaxFailures = 2
	replicaA := newScriptedSender("A", nil, responseStep{status: 500})
	replicaB := newScriptedSender("B", nil, responseStep{status: 500})
	replicaC := newScriptedSender("C", nil, responseStep{status: 200})
	strategy := resilience.RoundRobin{}

	// When:
	_, err := strategy.Do(context.Background(), []resilience.Sender{replicaA, replicaB, replicaC}, limits)

	// Then:
	if !errors.Is(err, resilience.ErrFailuresExhausted) {
		t.Fatalf("error = %v", err)
	}
	replicaA.assertCalls(t, 1, 0)
	replicaB.assertCalls(t, 1, 0)
	replicaC.assertCalls(t, 0, 0)
}

func TestRoundRobinDefaultComposition(t *testing.T) {
	// Given:
	replicaA := newScriptedSender("A", nil, responseStep{status: 500})
	replicaB := newScriptedSender("B", nil, responseStep{status: 200})
	strategy := resilience.RoundRobin{Strategy: resilience.Default{}}

	// When:
	_, err := strategy.Do(context.Background(), []resilience.Sender{replicaA, replicaB}, testBudgets())

	// Then:
	var statusErr *resilience.StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != 500 {
		t.Fatalf("error = %v", err)
	}
	replicaA.assertCalls(t, 1, 0)
	replicaB.assertCalls(t, 0, 0)
}

func TestRoundRobinSingleReplica(t *testing.T) {
	// Given:
	replicaA := newScriptedSender("A", nil, responseStep{status: 500}, responseStep{status: 200})
	strategy := resilience.RoundRobin{}

	// When:
	response, err := strategy.Do(context.Background(), []resilience.Sender{replicaA}, testBudgets())

	// Then:
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if response.StatusCode != 200 {
		t.Errorf("unexpected response: %+v", response)
	}
	replicaA.assertCalls(t, 2, 0)
}

func TestRoundRobinSharedDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		limits := testBudgets()
		limits.Latency = 150 * time.Millisecond
		replicaA := newScriptedSender("A", nil, responseStep{hang: true})
		replicaB := newScriptedSender("B", nil, responseStep{hang: true})
		replicaC := newScriptedSender("C", nil, responseStep{status: 200})
		start := time.Now()
		strategy := resilience.RoundRobin{}

		// When:
		_, err := strategy.Do(context.Background(), []resilience.Sender{replicaA, replicaB, replicaC}, limits)

		// Then:
		if !errors.Is(err, resilience.ErrLatencyExhausted) {
			t.Fatalf("Do() error = %v, want latency exhaustion", err)
		}
		if elapsed := time.Since(start); elapsed != limits.Latency {
			t.Errorf("elapsed = %v, want %v", elapsed, limits.Latency)
		}
		replicaA.assertCalls(t, 1, 1)
		replicaB.assertCalls(t, 1, 1)
		replicaC.assertCalls(t, 0, 0)
	})
}
