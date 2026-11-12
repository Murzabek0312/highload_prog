package resilience_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Murzabek0312/highload_prog/hw2/resilience"
)

type responseStep struct {
	status int
	body   string
	delay  time.Duration
	hang   bool
	err    error
}

type recordedCall struct {
	name string
	at   time.Time
}

type callLog struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (l *callLog) record(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, recordedCall{name: name, at: time.Now()})
}

func (l *callLog) snapshot() []recordedCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]recordedCall(nil), l.calls...)
}

// Сценарий и задержки задаются в тестовом отправителе, а не через API Sender.Send.
type scriptedSender struct {
	mu       sync.Mutex
	name     string
	steps    []responseStep
	log      *callLog
	calls    int
	active   int
	canceled int
}

func newScriptedSender(name string, log *callLog, steps ...responseStep) *scriptedSender {
	return &scriptedSender{
		name:  name,
		log:   log,
		steps: steps,
	}
}

func (f *scriptedSender) Send(ctx context.Context) (resilience.Response, error) {
	f.mu.Lock()
	index := f.calls
	f.calls++
	f.active++
	f.mu.Unlock()
	if f.log != nil {
		f.log.record(f.name)
	}
	defer func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.active--
		if ctx.Err() != nil {
			f.canceled++
		}
	}()
	if index >= len(f.steps) {
		return resilience.Response{}, fmt.Errorf("unexpected call %d to %s", index+1, f.name)
	}
	step := f.steps[index]
	if step.hang {
		<-ctx.Done()
		return resilience.Response{}, ctx.Err()
	}
	if step.delay > 0 {
		timer := time.NewTimer(step.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return resilience.Response{}, ctx.Err()
		case <-timer.C:
		}
	}
	return resilience.Response{StatusCode: step.status, Body: []byte(step.body)}, step.err
}

func (f *scriptedSender) assertCalls(t *testing.T, wantCalls, wantCanceled int) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != wantCalls {
		t.Errorf("%s: calls = %d, want %d", f.name, f.calls, wantCalls)
	}
	if f.canceled != wantCanceled {
		t.Errorf("%s: canceled calls = %d, want %d", f.name, f.canceled, wantCanceled)
	}
	if f.active != 0 {
		t.Errorf("%s: active calls = %d, want 0", f.name, f.active)
	}
}

func testBudgets() resilience.Budgets {
	return resilience.Budgets{
		MaxAttempts:       10,
		MaxFastErrors:     10,
		MaxFailures:       10,
		MaxTimeouts:       10,
		Latency:           time.Second,
		SubrequestLatency: 100 * time.Millisecond,
	}
}

type expectedCall struct {
	name  string
	after time.Duration
}

func assertCallSequence(t *testing.T, log *callLog, start time.Time, want []expectedCall) {
	t.Helper()
	calls := log.snapshot()
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i, got := range calls {
		if got.name != want[i].name || got.at.Sub(start) != want[i].after {
			t.Errorf("call %d = %s at %v, want %s at %v", i, got.name, got.at.Sub(start), want[i].name, want[i].after)
		}
	}
}

type senderFunc func(context.Context) (resilience.Response, error)

func (f senderFunc) Send(ctx context.Context) (resilience.Response, error) {
	return f(ctx)
}
