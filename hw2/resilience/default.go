package resilience

import "context"

// Default выполняет ровно одну попытку, если проверка параметров и внешняя отмена
// не препятствуют запуску. Счетчики повторов в Budgets здесь не используются.
type Default struct{}

var _ Client = Default{}

// Do выполняет одну попытку с заданными ограничениями времени.
func (Default) Do(ctx context.Context, sender Sender, budgets Budgets) (Response, error) {
	if err := validateSenders([]Sender{sender}); err != nil {
		return Response{}, err
	}
	if err := budgets.validateTime(); err != nil {
		return Response{}, err
	}
	ctx, cancel := context.WithTimeoutCause(ctx, budgets.Latency, ErrLatencyExhausted)
	defer cancel()

	response, _, err := sendAttempt(ctx, sender, budgets.SubrequestLatency)
	return response, err
}
