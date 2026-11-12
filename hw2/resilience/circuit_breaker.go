package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CircuitBreaker расширяет RoundRobin исключением проблемных реплик из ротации.
// После FailureThreshold ошибок подряд (4xx, 5xx или таймаут попытки) реплика
// исключается на время Cooldown, затем возвращается со сброшенным счетчиком.
// Успешный ответ сбрасывает счетчик ошибок реплики.
//
// Состояние общее для всех вызовов Do одного CircuitBreaker и привязано к позиции
// реплики в списке отправителей: для каждого запроса создаются новые Sender, поэтому
// одна и та же реплика должна занимать одну и ту же позицию. Если исключены все
// реплики, запрос сразу завершается ошибкой ErrAllReplicasOpen.
//
// CircuitBreaker нельзя копировать после первого использования.
type CircuitBreaker struct {
	FailureThreshold int
	Cooldown         time.Duration
	// Strategy — последовательная стратегия повторов, как в RoundRobin; nil означает Retry.
	Strategy Client

	mu       sync.Mutex
	replicas []replicaState
}

var _ MultiClient = (*CircuitBreaker)(nil)

type replicaState struct {
	errorCount int
	openUntil  time.Time
}

// Do выполняет запрос по кругу, пропуская временно исключенные реплики.
func (b *CircuitBreaker) Do(ctx context.Context, senders []Sender, budgets Budgets) (Response, error) {
	if b.FailureThreshold <= 0 || b.Cooldown <= 0 {
		return Response{}, fmt.Errorf("%w: breaker threshold and cooldown must be positive", ErrInvalidConfig)
	}
	return RoundRobin{Strategy: b.Strategy}.do(ctx, senders, budgets, b)
}

func (b *CircuitBreaker) allow(index int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.replica(index)
	if state.openUntil.IsZero() {
		return true
	}
	if time.Now().Before(state.openUntil) {
		return false
	}
	*state = replicaState{}
	return true
}

func (b *CircuitBreaker) record(ctx context.Context, index int, response Response, err error) {
	failed, counted := classifyForBreaker(ctx, response, err)
	if !counted {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.replica(index)
	if !failed {
		state.errorCount = 0
		return
	}
	state.errorCount++
	if state.errorCount >= b.FailureThreshold {
		state.openUntil = time.Now().Add(b.Cooldown)
	}
}

// replica возвращает состояние реплики, расширяя список при необходимости.
// Вызывается под b.mu.
func (b *CircuitBreaker) replica(index int) *replicaState {
	if index >= len(b.replicas) {
		b.replicas = append(b.replicas, make([]replicaState, index+1-len(b.replicas))...)
	}
	return &b.replicas[index]
}

// classifyForBreaker определяет, считается ли результат ошибкой реплики.
// Внешняя отмена и неожиданные ошибки отправителя на состояние реплики не влияют.
func classifyForBreaker(ctx context.Context, response Response, err error) (failed, counted bool) {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return true, true
	case ctx.Err() != nil || err != nil:
		return false, false
	case response.StatusCode >= 200 && response.StatusCode < 300:
		return false, true
	case response.StatusCode >= 400 && response.StatusCode < 600:
		return true, true
	default:
		return false, false
	}
}
