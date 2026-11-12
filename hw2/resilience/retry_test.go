package resilience_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Murzabek0312/highload_prog/hw2/resilience"
)

func TestRetryMixedErrorsThenSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		limits := testBudgets()
		limits.MaxAttempts = 4
		limits.MaxFastErrors = 2
		limits.MaxFailures = 2
		limits.MaxTimeouts = 2
		sender := newScriptedSender("server", nil,
			responseStep{status: 400},
			responseStep{status: 503},
			responseStep{hang: true},
			responseStep{status: 200, body: "ok"},
		)

		// When:
		response, err := (resilience.Retry{}).Do(context.Background(), sender, limits)

		// Then:
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if string(response.Body) != "ok" {
			t.Errorf("unexpected response: %+v", response)
		}
		sender.assertCalls(t, 4, 1)
	})
}

func TestRetryErrorBudgets(t *testing.T) {
	tests := []struct {
		name           string
		setLimit       func(*resilience.Budgets, int)
		failedResponse responseStep
		wantErr        error
	}{
		{
			name: "fast",
			setLimit: func(limits *resilience.Budgets, limit int) {
				limits.MaxFastErrors = limit
			},
			failedResponse: responseStep{status: 400},
			wantErr:        resilience.ErrFastErrorsExhausted,
		},
		{
			name: "failure",
			setLimit: func(limits *resilience.Budgets, limit int) {
				limits.MaxFailures = limit
			},
			failedResponse: responseStep{status: 503},
			wantErr:        resilience.ErrFailuresExhausted,
		},
		{
			name: "timeout",
			setLimit: func(limits *resilience.Budgets, limit int) {
				limits.MaxTimeouts = limit
			},
			failedResponse: responseStep{hang: true},
			wantErr:        resilience.ErrTimeoutsExhausted,
		},
	}
	for _, test := range tests {
		for _, limit := range []int{0, 1, 2} {
			t.Run(test.name+"/"+strconv.Itoa(limit), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					// Given:
					limits := testBudgets()
					test.setLimit(&limits, limit)
					sender := newScriptedSender("server", nil,
						test.failedResponse,
						test.failedResponse,
						responseStep{status: 200},
					)

					// When:
					_, err := (resilience.Retry{}).Do(context.Background(), sender, limits)

					// Then:
					if !errors.Is(err, test.wantErr) {
						t.Fatalf("error = %v, want %v", err, test.wantErr)
					}
					calls := max(1, limit)
					canceled := 0
					if test.failedResponse.hang {
						canceled = calls
					}
					sender.assertCalls(t, calls, canceled)
				})
			})
		}
	}
}

func TestRetryAttemptBudget(t *testing.T) {
	// Given:
	limits := testBudgets()
	limits.MaxAttempts = 2
	sender := newScriptedSender("server", nil,
		responseStep{status: 400},
		responseStep{status: 503},
		responseStep{status: 200},
	)

	// When:
	response, err := (resilience.Retry{}).Do(context.Background(), sender, limits)

	// Then:
	if !errors.Is(err, resilience.ErrAttemptsExhausted) {
		t.Fatalf("Do() error = %v, want attempt budget exhaustion", err)
	}
	var statusErr *resilience.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("Do() error = %v, want StatusError", err)
	}
	if statusErr.StatusCode != 503 {
		t.Errorf("error status = %d, want 503", statusErr.StatusCode)
	}
	if response.StatusCode != 503 {
		t.Errorf("response status = %d, want 503", response.StatusCode)
	}
	sender.assertCalls(t, 2, 0)
}

func TestRetrySharedLatency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		limits := testBudgets()
		limits.Latency = 250 * time.Millisecond
		sender := newScriptedSender("server", nil,
			responseStep{hang: true},
			responseStep{hang: true},
			responseStep{hang: true},
			responseStep{status: 200},
		)
		start := time.Now()

		// When:
		_, err := (resilience.Retry{}).Do(context.Background(), sender, limits)

		// Then:
		if !errors.Is(err, resilience.ErrLatencyExhausted) || errors.Is(err, resilience.ErrTimeoutsExhausted) {
			t.Fatalf("error = %v", err)
		}
		if time.Since(start) != limits.Latency {
			t.Fatalf("elapsed = %v", time.Since(start))
		}
		sender.assertCalls(t, 3, 3)
	})
}

func TestRetryStopsOnCallerCancellation(t *testing.T) {
	// Given:
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	sender := senderFunc(func(context.Context) (resilience.Response, error) {
		calls++
		cancel()
		return resilience.Response{StatusCode: 500}, nil
	})

	// When:
	_, err := (resilience.Retry{}).Do(ctx, sender, testBudgets())

	// Then:
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want cancellation", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}
