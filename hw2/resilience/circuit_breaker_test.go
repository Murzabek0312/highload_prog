package resilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Murzabek0312/highload_prog/hw2/resilience"
)

func TestCircuitBreakerExcludesReplicaAcrossRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		breaker := &resilience.CircuitBreaker{FailureThreshold: 1, Cooldown: 50 * time.Millisecond}
		firstA := newScriptedSender("A", nil, responseStep{status: 500})
		firstB := newScriptedSender("B", nil, responseStep{status: 200})
		_, err := breaker.Do(context.Background(), []resilience.Sender{firstA, firstB}, testBudgets())
		if err != nil {
			t.Fatalf("first Do() error = %v", err)
		}
		secondA := newScriptedSender("A", nil)
		secondB := newScriptedSender("B", nil, responseStep{status: 200})

		// When:
		response, err := breaker.Do(context.Background(), []resilience.Sender{secondA, secondB}, testBudgets())

		// Then:
		if err != nil {
			t.Fatalf("second Do() error = %v", err)
		}
		if response.StatusCode != 200 {
			t.Errorf("response status = %d, want 200", response.StatusCode)
		}
		// Реплика A исключена после ошибки в первом запросе.
		secondA.assertCalls(t, 0, 0)
		secondB.assertCalls(t, 1, 0)
	})
}

func TestCircuitBreakerReturnsReplicaAfterCooldown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		breaker := &resilience.CircuitBreaker{FailureThreshold: 2, Cooldown: 50 * time.Millisecond}
		first := newScriptedSender("server", nil, responseStep{status: 500}, responseStep{status: 500})
		_, err := breaker.Do(context.Background(), []resilience.Sender{first}, testBudgets())
		if !errors.Is(err, resilience.ErrAllReplicasOpen) {
			t.Fatalf("first Do() error = %v, want all replicas open", err)
		}
		time.Sleep(50 * time.Millisecond)
		second := newScriptedSender("server", nil, responseStep{status: 500}, responseStep{status: 200})

		// When:
		response, err := breaker.Do(context.Background(), []resilience.Sender{second}, testBudgets())

		// Then:
		if err != nil {
			t.Fatalf("second Do() error = %v", err)
		}
		if response.StatusCode != 200 {
			t.Errorf("response status = %d, want 200", response.StatusCode)
		}
		// После возврата одна ошибка не должна повторно отключать реплику при K=2.
		first.assertCalls(t, 2, 0)
		second.assertCalls(t, 2, 0)
	})
}

func TestCircuitBreakerSuccessResetsErrorCount(t *testing.T) {
	// Given:
	breaker := &resilience.CircuitBreaker{FailureThreshold: 2, Cooldown: time.Second}
	first := newScriptedSender("server", nil, responseStep{status: 500}, responseStep{status: 200})
	_, err := breaker.Do(context.Background(), []resilience.Sender{first}, testBudgets())
	if err != nil {
		t.Fatalf("first Do() error = %v", err)
	}
	second := newScriptedSender("server", nil, responseStep{status: 500}, responseStep{status: 200})

	// When:
	response, err := breaker.Do(context.Background(), []resilience.Sender{second}, testBudgets())

	// Then:
	// Без сброса после успеха вторая ошибка подряд отключила бы реплику.
	if err != nil {
		t.Fatalf("second Do() error = %v", err)
	}
	if response.StatusCode != 200 {
		t.Errorf("response status = %d, want 200", response.StatusCode)
	}
	second.assertCalls(t, 2, 0)
}

func TestCircuitBreakerCountsAttemptTimeouts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		breaker := &resilience.CircuitBreaker{FailureThreshold: 1, Cooldown: time.Second}
		limits := testBudgets()
		limits.SubrequestLatency = 10 * time.Millisecond
		first := newScriptedSender("server", nil, responseStep{hang: true})
		second := newScriptedSender("server", nil)
		start := time.Now()

		// When:
		_, firstErr := breaker.Do(context.Background(), []resilience.Sender{first}, limits)
		_, secondErr := breaker.Do(context.Background(), []resilience.Sender{second}, limits)

		// Then:
		if !errors.Is(firstErr, resilience.ErrAllReplicasOpen) {
			t.Fatalf("first Do() error = %v, want all replicas open", firstErr)
		}
		// Все реплики исключены: запрос завершается сразу, без ожидания cooldown.
		if !errors.Is(secondErr, resilience.ErrAllReplicasOpen) {
			t.Fatalf("second Do() error = %v, want all replicas open", secondErr)
		}
		if elapsed := time.Since(start); elapsed != limits.SubrequestLatency {
			t.Errorf("elapsed = %v, want %v", elapsed, limits.SubrequestLatency)
		}
		first.assertCalls(t, 1, 1)
		second.assertCalls(t, 0, 0)
	})
}

func TestCircuitBreakerWithBackoffStrategy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Given:
		log := &callLog{}
		breaker := &resilience.CircuitBreaker{
			FailureThreshold: 1,
			Cooldown:         time.Second,
			Strategy:         resilience.Backoff{InitialDelay: 10 * time.Millisecond, Multiplier: 2},
		}
		replicaA := newScriptedSender("A", log, responseStep{status: 500})
		replicaB := newScriptedSender("B", log, responseStep{status: 503})
		start := time.Now()

		// When:
		response, err := breaker.Do(context.Background(), []resilience.Sender{replicaA, replicaB}, testBudgets())

		// Then:
		if !errors.Is(err, resilience.ErrAllReplicasOpen) {
			t.Fatalf("Do() error = %v, want all replicas open", err)
		}
		if response.StatusCode != 0 {
			t.Errorf("response status = %d, want empty response", response.StatusCode)
		}
		// Backoff задерживает переход к следующей реплике; после двух ошибок пул пуст.
		assertCallSequence(t, log, start, []expectedCall{
			{name: "A"},
			{name: "B", after: 10 * time.Millisecond},
		})
		replicaA.assertCalls(t, 1, 0)
		replicaB.assertCalls(t, 1, 0)
	})
}

func TestCircuitBreakerIsSafeForConcurrentRequests(t *testing.T) {
	// Given:
	breaker := &resilience.CircuitBreaker{FailureThreshold: 3, Cooldown: time.Second}
	var requests sync.WaitGroup

	// When:
	for range 8 {
		requests.Go(func() {
			replicaA := newScriptedSender("A", nil, responseStep{status: 500}, responseStep{status: 500})
			replicaB := newScriptedSender("B", nil, responseStep{status: 200}, responseStep{status: 200})
			_, _ = breaker.Do(context.Background(), []resilience.Sender{replicaA, replicaB}, testBudgets())
		})
	}
	requests.Wait()

	// Then:
	// Гонки обнаруживает race detector; после серии ошибок A должна быть исключена.
	replicaA := newScriptedSender("A", nil)
	replicaB := newScriptedSender("B", nil, responseStep{status: 200})
	if _, err := breaker.Do(context.Background(), []resilience.Sender{replicaA, replicaB}, testBudgets()); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	replicaA.assertCalls(t, 0, 0)
}
