package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Retry повторяет неуспешные попытки без задержки.
type Retry struct{}

var _ Client = Retry{}

// Do повторяет попытки до успеха, отмены или исчерпания бюджета.
func (Retry) Do(ctx context.Context, sender Sender, budgets Budgets) (Response, error) {
	if err := validateSenders([]Sender{sender}); err != nil {
		return Response{}, err
	}
	loop := retryLoop{sender: sender}
	return loop.run(ctx, budgets)
}

// retryDelay возвращает паузу перед следующей попыткой по результату
// последней попытки и уже учтенным ошибкам.
type retryDelay func(result outcome, counts errorCounts) time.Duration

// retryLoop выполняет последовательные попытки и учитывает бюджеты.
// Политика задержки задается стратегией; nil означает повтор без паузы.
type retryLoop struct {
	sender Sender
	delay  retryDelay
}

func (r retryLoop) run(ctx context.Context, budgets Budgets) (Response, error) {
	if err := budgets.validateRetries(); err != nil {
		return Response{}, err
	}
	ctx, cancel := context.WithTimeoutCause(ctx, budgets.Latency, ErrLatencyExhausted)
	defer cancel()

	var counts errorCounts

	for attempts := 1; ; attempts++ {
		response, result, err := sendAttempt(ctx, r.sender, budgets.SubrequestLatency)
		if result == outcomeFinal {
			return response, err
		}

		exhausted := counts.record(result, budgets)
		if attempts >= budgets.MaxAttempts {
			exhausted = errors.Join(exhausted, ErrAttemptsExhausted)
		}
		if exhausted != nil {
			return response, errors.Join(exhausted, err)
		}

		if r.delay == nil {
			continue
		}
		delay := r.delay(result, counts)
		// Попытка после deadline невозможна, поэтому не ждем его впустую.
		if !fitsBeforeDeadline(ctx, delay) {
			exhausted := fmt.Errorf("%w: retry delay %v exceeds remaining time", ErrLatencyExhausted, delay)
			return response, errors.Join(exhausted, err)
		}
		if err := wait(ctx, delay); err != nil {
			return Response{}, err
		}
	}
}
