// Package resilience реализует стратегии устойчивости из DDIA HW2.
package resilience

import (
	"context"
	"fmt"
)

// Response содержит статус и тело ответа сервера. Поддерживаются статусы
// 2xx, 4xx и 5xx; таймаут определяет клиент, а не сервер.
type Response struct {
	StatusCode int
	Body       []byte
}

// Sender обслуживает попытки одного логического запроса. Send должен завершаться
// при отмене ctx. Один отправитель нельзя использовать для разных запросов.
type Sender interface {
	Send(ctx context.Context) (Response, error)
}

// Client выполняет один логический запрос через одного отправителя.
type Client interface {
	Do(ctx context.Context, sender Sender, budgets Budgets) (Response, error)
}

// MultiClient выполняет один логический запрос к набору реплик.
type MultiClient interface {
	Do(ctx context.Context, senders []Sender, budgets Budgets) (Response, error)
}

func validateSenders(senders []Sender) error {
	if len(senders) == 0 {
		return fmt.Errorf("%w: sender list is empty", ErrInvalidConfig)
	}
	for i, sender := range senders {
		if sender == nil {
			return fmt.Errorf("%w: sender %d is nil", ErrInvalidConfig, i)
		}
	}
	return nil
}
