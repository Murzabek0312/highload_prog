package resilience

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Backoff задерживает повторы после 5xx и таймаутов отдельных попыток.
// Ответ 4xx не увеличивает и не сбрасывает задержку для следующей тяжелой ошибки.
type Backoff struct {
	InitialDelay time.Duration
	Multiplier   float64
}

var _ Client = Backoff{}

// Do повторяет запросы с экспоненциальной задержкой после ошибок сервера и таймаутов.
func (b Backoff) Do(ctx context.Context, sender Sender, budgets Budgets) (Response, error) {
	if err := validateSenders([]Sender{sender}); err != nil {
		return Response{}, err
	}
	if b.InitialDelay <= 0 || b.Multiplier <= 1 || math.IsNaN(b.Multiplier) || math.IsInf(b.Multiplier, 0) {
		return Response{}, fmt.Errorf("%w: backoff needs a positive delay and a finite multiplier greater than one", ErrInvalidConfig)
	}
	loop := retryLoop{
		sender: sender,
		delay:  b.delay,
	}
	return loop.run(ctx, budgets)
}

func (b Backoff) delay(result outcome, counts errorCounts) time.Duration {
	if result == outcomeFastError {
		return 0
	}
	heavyErrors := counts.failures + counts.timeouts
	// Считаем от исходной задержки: повторное умножение округленного значения
	// может остановить рост при дробном множителе, например для 1ns * 1.5.
	next := float64(b.InitialDelay) * math.Pow(b.Multiplier, float64(heavyErrors-1))
	if next >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(next)
}
