package resilience_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Murzabek0312/highload_prog/hw2/resilience"
)

func TestDefaultResponse(t *testing.T) {
	for _, status := range []int{200, 204, 299, 400, 499, 500, 599} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			// Given:
			sender := newScriptedSender("server", nil, responseStep{status: status, body: "payload"})

			// When:
			response, err := (resilience.Default{}).Do(context.Background(), sender, testBudgets())

			// Then:
			if response.StatusCode != status || string(response.Body) != "payload" {
				t.Fatalf("response = %+v", response)
			}
			if status < 300 && err != nil {
				t.Fatal(err)
			}
			if status >= 400 {
				var statusErr *resilience.StatusError
				if !errors.As(err, &statusErr) || statusErr.StatusCode != status {
					t.Fatalf("error = %v", err)
				}
			}
			sender.assertCalls(t, 1, 0)
		})
	}
}

func TestDefaultTimeouts(t *testing.T) {
	tests := []struct {
		name              string
		subrequestLatency time.Duration
		wantElapsed       time.Duration
		wantErr           error
	}{
		{name: "remaining total budget", wantElapsed: time.Second, wantErr: resilience.ErrLatencyExhausted},
		{
			name:              "shorter attempt budget",
			subrequestLatency: 100 * time.Millisecond,
			wantElapsed:       100 * time.Millisecond,
			wantErr:           resilience.ErrSubrequestTimeout,
		},
		{
			name:              "attempt clipped by total budget",
			subrequestLatency: 2 * time.Second,
			wantElapsed:       time.Second,
			wantErr:           resilience.ErrLatencyExhausted,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Given:
				limits := testBudgets()
				limits.SubrequestLatency = test.subrequestLatency
				sender := newScriptedSender("server", nil, responseStep{hang: true})
				start := time.Now()

				// When:
				_, err := (resilience.Default{}).Do(context.Background(), sender, limits)

				// Then:
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Do() error = %v, want %v", err, test.wantErr)
				}
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("Do() error = %v, want context deadline", err)
				}
				if count := strings.Count(err.Error(), test.wantErr.Error()); count != 1 {
					t.Errorf("error cause appears %d times, want 1: %v", count, err)
				}
				if elapsed := time.Since(start); elapsed != test.wantElapsed {
					t.Errorf("elapsed = %v, want %v", elapsed, test.wantElapsed)
				}
				sender.assertCalls(t, 1, 1)
			})
		})
	}
}

func TestDefaultCallerCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sender := newScriptedSender("server", nil, responseStep{hang: true})
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		start := time.Now()

		// When:
		_, err := (resilience.Default{}).Do(ctx, sender, testBudgets())

		// Then:
		if !errors.Is(err, context.Canceled) || errors.Is(err, resilience.ErrSubrequestTimeout) {
			t.Fatalf("error = %v", err)
		}
		if time.Since(start) != 20*time.Millisecond {
			t.Fatalf("elapsed = %v", time.Since(start))
		}
		sender.assertCalls(t, 1, 1)
	})
}

func TestDefaultUnexpectedResult(t *testing.T) {
	transportErr := errors.New("transport failed")
	for _, test := range []struct {
		name         string
		responseStep responseStep
		want         error
	}{
		{"unsupported status", responseStep{status: 302}, resilience.ErrInvalidResponse},
		{"transport error", responseStep{err: transportErr}, transportErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given:
			sender := newScriptedSender("server", nil, test.responseStep)

			// When:
			_, err := (resilience.Default{}).Do(context.Background(), sender, testBudgets())

			// Then:
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v", err)
			}
			sender.assertCalls(t, 1, 0)
		})
	}
}
