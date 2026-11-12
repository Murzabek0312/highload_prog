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

func TestInvalidBudgetsDoNotSend(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*resilience.Budgets)
	}{
		{"zero attempts", func(limits *resilience.Budgets) { limits.MaxAttempts = 0 }},
		{"negative attempts", func(limits *resilience.Budgets) { limits.MaxAttempts = -1 }},
		{"negative fast errors", func(limits *resilience.Budgets) { limits.MaxFastErrors = -1 }},
		{"negative failures", func(limits *resilience.Budgets) { limits.MaxFailures = -1 }},
		{"negative timeouts", func(limits *resilience.Budgets) { limits.MaxTimeouts = -1 }},
		{"zero latency", func(limits *resilience.Budgets) { limits.Latency = 0 }},
		{"negative latency", func(limits *resilience.Budgets) { limits.Latency = -1 }},
		{"negative subrequest latency", func(limits *resilience.Budgets) { limits.SubrequestLatency = -1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given:
			limits := testBudgets()
			test.change(&limits)
			sender := newScriptedSender("server", nil, responseStep{status: 200})

			// When:
			_, err := (resilience.Retry{}).Do(context.Background(), sender, limits)

			// Then:
			if !errors.Is(err, resilience.ErrInvalidConfig) {
				t.Fatalf("error = %v", err)
			}
			sender.assertCalls(t, 0, 0)
		})
	}
}

func TestInvalidStrategiesDoNotSend(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(context.Context, resilience.Sender) (resilience.Response, error)
	}{
		{"nil sender", func(ctx context.Context, _ resilience.Sender) (resilience.Response, error) {
			return (resilience.Default{}).Do(ctx, nil, testBudgets())
		}},
		{"empty pool", func(ctx context.Context, _ resilience.Sender) (resilience.Response, error) {
			return (resilience.RoundRobin{}).Do(ctx, nil, testBudgets())
		}},
		{"nil replica", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Hedging{}).Do(ctx, []resilience.Sender{sender, nil}, testBudgets())
		}},
		{"zero backoff delay", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Backoff{Multiplier: 2}).Do(ctx, sender, testBudgets())
		}},
		{"unit multiplier", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Backoff{InitialDelay: time.Second, Multiplier: 1}).Do(ctx, sender, testBudgets())
		}},
		{"NaN multiplier", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Backoff{InitialDelay: time.Second, Multiplier: math.NaN()}).Do(ctx, sender, testBudgets())
		}},
		{"infinite multiplier", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Backoff{InitialDelay: time.Second, Multiplier: math.Inf(1)}).Do(ctx, sender, testBudgets())
		}},
		{"negative hedge delay", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Hedging{Delay: -1}).Do(ctx, []resilience.Sender{sender}, testBudgets())
		}},
		{"zero breaker threshold", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (&resilience.CircuitBreaker{Cooldown: time.Second}).Do(ctx, []resilience.Sender{sender}, testBudgets())
		}},
		{"zero breaker cooldown", func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (&resilience.CircuitBreaker{FailureThreshold: 1}).Do(ctx, []resilience.Sender{sender}, testBudgets())
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given:
			sender := newScriptedSender("server", nil, responseStep{status: 200})

			// When:
			_, err := test.run(context.Background(), sender)

			// Then:
			if !errors.Is(err, resilience.ErrInvalidConfig) {
				t.Fatalf("error = %v", err)
			}
			sender.assertCalls(t, 0, 0)
		})
	}
}

func TestAlreadyCanceledRequestDoesNotSend(t *testing.T) {
	strategies := map[string]func(context.Context, resilience.Sender) (resilience.Response, error){
		"default": func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Default{}).Do(ctx, sender, testBudgets())
		},
		"retry": func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Retry{}).Do(ctx, sender, testBudgets())
		},
		"backoff": func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Backoff{InitialDelay: time.Second, Multiplier: 2}).Do(ctx, sender, testBudgets())
		},
		"round robin": func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.RoundRobin{}).Do(ctx, []resilience.Sender{sender}, testBudgets())
		},
		"hedging": func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (resilience.Hedging{Delay: time.Second}).Do(ctx, []resilience.Sender{sender}, testBudgets())
		},
		"breaker": func(ctx context.Context, sender resilience.Sender) (resilience.Response, error) {
			return (&resilience.CircuitBreaker{FailureThreshold: 1, Cooldown: time.Second}).Do(ctx, []resilience.Sender{sender}, testBudgets())
		},
	}
	for name, run := range strategies {
		t.Run(name, func(t *testing.T) {
			// Given:
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			sender := newScriptedSender("server", nil, responseStep{status: 200})

			// When:
			_, err := run(ctx, sender)

			// Then:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v", err)
			}
			sender.assertCalls(t, 0, 0)
		})
	}
}

func TestCallerDeadlineTakesPrecedence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		sender := newScriptedSender("server", nil, responseStep{hang: true}, responseStep{status: 200})
		start := time.Now()

		// When:
		_, err := (resilience.Retry{}).Do(ctx, sender, testBudgets())

		// Then:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Do() error = %v, want caller deadline", err)
		}
		if errors.Is(err, resilience.ErrLatencyExhausted) || errors.Is(err, resilience.ErrSubrequestTimeout) {
			t.Errorf("caller deadline was classified as a strategy timeout: %v", err)
		}
		if time.Since(start) != 20*time.Millisecond {
			t.Fatalf("elapsed = %v", time.Since(start))
		}
		sender.assertCalls(t, 1, 1)
	})
}

func TestResponseAtDeadlineIsNotAccepted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		limits := testBudgets()
		limits.SubrequestLatency = 0
		sender := senderFunc(func(context.Context) (resilience.Response, error) {
			time.Sleep(limits.Latency)
			return resilience.Response{StatusCode: 200}, nil
		})

		// When:
		_, err := (resilience.Default{}).Do(context.Background(), sender, limits)

		// Then:
		if !errors.Is(err, resilience.ErrLatencyExhausted) {
			t.Fatalf("error = %v", err)
		}
	})
}
