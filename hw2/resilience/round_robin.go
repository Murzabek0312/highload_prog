package resilience

import "context"

// RoundRobin применяет последовательный Client к отправителю, выбирающему реплики
// по кругу. При nil в Strategy используется Retry. Можно подставить Backoff
// или Default без вложенных циклов повторов.
type RoundRobin struct {
	Strategy Client
}

var _ MultiClient = RoundRobin{}

// Do обходит отправителей по кругу, используя одну стратегию на весь запрос.
func (r RoundRobin) Do(ctx context.Context, senders []Sender, budgets Budgets) (Response, error) {
	return r.do(ctx, senders, budgets, nil)
}

// do позволяет расширениям round-robin, например circuit breaker, исключать
// реплики из ротации и получать результаты попыток.
func (r RoundRobin) do(ctx context.Context, senders []Sender, budgets Budgets, guard replicaGuard) (Response, error) {
	if err := validateSenders(senders); err != nil {
		return Response{}, err
	}

	strategy := r.Strategy
	if strategy == nil {
		strategy = Retry{}
	}
	rotating := &rotatingSender{senders: senders, guard: guard}
	return strategy.Do(ctx, rotating, budgets)
}

// replicaGuard решает, доступна ли реплика, и учитывает результат обращения к ней.
type replicaGuard interface {
	allow(index int) bool
	record(ctx context.Context, index int, response Response, err error)
}

type rotatingSender struct {
	senders   []Sender
	guard     replicaGuard
	nextIndex int
}

func (r *rotatingSender) Send(ctx context.Context) (Response, error) {
	index, ok := r.pick()
	if !ok {
		return Response{}, ErrAllReplicasOpen
	}
	response, err := r.senders[index].Send(ctx)
	if r.guard != nil {
		r.guard.record(ctx, index, response, err)
	}
	return response, err
}

// pick выбирает следующую по кругу доступную реплику.
func (r *rotatingSender) pick() (int, bool) {
	for offset := range r.senders {
		index := (r.nextIndex + offset) % len(r.senders)
		if r.guard == nil || r.guard.allow(index) {
			r.nextIndex = (index + 1) % len(r.senders)
			return index, true
		}
	}
	return 0, false
}
