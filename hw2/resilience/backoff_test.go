package resilience_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Murzabek0312/highload_prog/hw2/resilience"
)

func TestBackoffMixedErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		limits := testBudgets()
		limits.SubrequestLatency = 20 * time.Millisecond
		log := &callLog{}
		sender := newScriptedSender("server", log,
			responseStep{status: 500},
			responseStep{hang: true},
			responseStep{status: 400},
			responseStep{status: 503},
			responseStep{status: 200},
		)
		strategy := resilience.Backoff{InitialDelay: 10 * time.Millisecond, Multiplier: 2}
		start := time.Now()

		// When:
		response, err := strategy.Do(context.Background(), sender, limits)

		// Then:
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if response.StatusCode != 200 {
			t.Errorf("unexpected response: %+v", response)
		}
		assertCallSequence(t, log, start, []expectedCall{
			{name: "server"},
			{name: "server", after: 10 * time.Millisecond},
			{name: "server", after: 50 * time.Millisecond},
			{name: "server", after: 50 * time.Millisecond},
			{name: "server", after: 90 * time.Millisecond},
		})
		sender.assertCalls(t, 5, 1)
	})
}

func TestBackoffFastErrorsHaveNoDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		sender := newScriptedSender("server", nil,
			responseStep{status: 400},
			responseStep{status: 401},
			responseStep{status: 200},
		)
		start := time.Now()
		strategy := resilience.Backoff{InitialDelay: time.Second, Multiplier: 2}

		// When:
		_, err := strategy.Do(context.Background(), sender, testBudgets())

		// Then:
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Errorf("elapsed = %v, want no delay", elapsed)
		}
		sender.assertCalls(t, 3, 0)
	})
}

func TestBackoffFractionalMultiplier(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		log := &callLog{}
		sender := newScriptedSender("server", log,
			responseStep{status: 500},
			responseStep{status: 500},
			responseStep{status: 500},
			responseStep{status: 200},
		)
		strategy := resilience.Backoff{InitialDelay: time.Nanosecond, Multiplier: 1.5}
		start := time.Now()

		// When:
		_, err := strategy.Do(context.Background(), sender, testBudgets())

		// Then:
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		assertCallSequence(t, log, start, []expectedCall{
			{name: "server"},
			{name: "server", after: time.Nanosecond},
			{name: "server", after: 2 * time.Nanosecond},
			{name: "server", after: 4 * time.Nanosecond},
		})
		sender.assertCalls(t, 4, 0)
	})
}

func TestBackoffWaitIsCancelable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		sender := newScriptedSender("server", nil,
			responseStep{status: 500},
			responseStep{status: 200},
		)
		strategy := resilience.Backoff{InitialDelay: 50 * time.Millisecond, Multiplier: 2}
		start := time.Now()

		// When:
		_, err := strategy.Do(ctx, sender, testBudgets())

		// Then:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Do() error = %v, want cancellation", err)
		}
		if elapsed := time.Since(start); elapsed != 20*time.Millisecond {
			t.Errorf("elapsed = %v, want 20ms", elapsed)
		}
		sender.assertCalls(t, 1, 0)
	})
}

func TestBackoffDoesNotWaitPastDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		limits := testBudgets()
		limits.Latency = 50 * time.Millisecond
		sender := newScriptedSender("server", nil, responseStep{status: 500}, responseStep{status: 200})
		strategy := resilience.Backoff{InitialDelay: time.Second, Multiplier: 2}
		start := time.Now()

		// When:
		response, err := strategy.Do(context.Background(), sender, limits)

		// Then:
		// Задержка длиннее остатка бюджета: ждать бессмысленно, последний ответ сохраняется.
		if !errors.Is(err, resilience.ErrLatencyExhausted) {
			t.Fatalf("Do() error = %v, want latency exhaustion", err)
		}
		var statusErr *resilience.StatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != 500 {
			t.Errorf("Do() error = %v, want last status 500", err)
		}
		if response.StatusCode != 500 {
			t.Errorf("response status = %d, want 500", response.StatusCode)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Errorf("elapsed = %v, want no delay", elapsed)
		}
		sender.assertCalls(t, 1, 0)
	})
}

func TestBackoffDoesNotWaitAfterLastAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		limits := testBudgets()
		limits.MaxAttempts = 1
		sender := newScriptedSender("server", nil, responseStep{status: 500})
		start := time.Now()
		strategy := resilience.Backoff{InitialDelay: time.Second, Multiplier: 2}

		// When:
		_, err := strategy.Do(context.Background(), sender, limits)

		// Then:
		if !errors.Is(err, resilience.ErrAttemptsExhausted) {
			t.Fatalf("Do() error = %v, want attempt budget exhaustion", err)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Errorf("elapsed = %v, want no delay", elapsed)
		}
		sender.assertCalls(t, 1, 0)
	})
}

func TestBackoffDurationDoesNotOverflow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		sender := newScriptedSender("server", nil,
			responseStep{status: 500},
			responseStep{status: 500},
			responseStep{status: 200},
		)
		start := time.Now()
		strategy := resilience.Backoff{InitialDelay: time.Nanosecond, Multiplier: math.MaxFloat64}

		// When:
		_, err := strategy.Do(context.Background(), sender, testBudgets())

		// Then:
		if !errors.Is(err, resilience.ErrLatencyExhausted) {
			t.Fatalf("Do() error = %v, want latency exhaustion", err)
		}
		// Переполненная задержка не помещается в бюджет: ожидания нет.
		if elapsed := time.Since(start); elapsed != time.Nanosecond {
			t.Errorf("elapsed = %v, want 1ns", elapsed)
		}
		sender.assertCalls(t, 2, 0)
	})
}
