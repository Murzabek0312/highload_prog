package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Hedging сразу запускает первую реплику, а через Delay — остальные, если успеха
// еще нет. Счетчики повторов в Budgets не используются. При ошибках всех реплик
// Do продолжает ждать общего deadline или внешней отмены.
type Hedging struct {
	Delay time.Duration
}

var _ MultiClient = Hedging{}

// Do возвращает первый успешный ответ и отменяет остальные попытки.
func (h Hedging) Do(ctx context.Context, senders []Sender, budgets Budgets) (Response, error) {
	if err := validateSenders(senders); err != nil {
		return Response{}, err
	}
	if err := budgets.validateTime(); err != nil {
		return Response{}, err
	}
	if h.Delay < 0 {
		return Response{}, fmt.Errorf("%w: hedging delay must be nonnegative", ErrInvalidConfig)
	}

	ctx, cancel := context.WithTimeoutCause(ctx, budgets.Latency, ErrLatencyExhausted)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		workers.Wait()
	}()
	if err := contextError(ctx); err != nil {
		return Response{}, err
	}

	// Нужны только успешные ответы. Буфер позволяет каждой горутине завершиться,
	// даже если другой ответ уже победил и координатор больше не читает канал.
	successes := make(chan Response, len(senders))
	startAttempt := func(sender Sender) {
		workers.Go(func() {
			// Default наследует общий deadline, поэтому новая попытка не продлевает запрос.
			response, err := (Default{}).Do(ctx, sender, budgets)
			if err == nil {
				successes <- response
			}
		})
	}

	firstStarted := make(chan struct{})
	startAttempt(startSignalingSender{sender: senders[0], started: firstStarted})
	select {
	case <-firstStarted:
	case <-ctx.Done():
		return Response{}, contextError(ctx)
	}

	timer := time.NewTimer(h.Delay)
	defer timer.Stop()
	delayElapsed := timer.C

	for {
		select {
		case <-ctx.Done():
			return Response{}, contextError(ctx)
		case response := <-successes:
			if err := contextError(ctx); err != nil {
				return Response{}, err
			}
			return response, nil
		case <-delayElapsed:
			// Отключаем эту ветку после однократного запуска остальных реплик.
			delayElapsed = nil
			if err := contextError(ctx); err != nil {
				return Response{}, err
			}
			// Если таймер и успешный ответ готовы одновременно, выбираем ответ.
			select {
			case response := <-successes:
				return response, nil
			default:
			}
			for _, sender := range senders[1:] {
				startAttempt(sender)
			}
		}
	}
}

// startSignalingSender сигнализирует об отправке после проверок отмены в Default.
// Первый Send будет вызван, даже если запрос к другой реплике с нулевой задержкой
// успеет победить до того, как планировщик снова запустит первую горутину.
type startSignalingSender struct {
	sender  Sender
	started chan<- struct{}
}

func (s startSignalingSender) Send(ctx context.Context) (Response, error) {
	close(s.started)
	return s.sender.Send(ctx)
}
