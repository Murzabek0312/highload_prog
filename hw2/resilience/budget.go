package resilience

import (
	"fmt"
	"time"
)

// Budgets ограничивает весь логический запрос, а не каждую реплику отдельно.
// Default и Hedging используют только два ограничения времени.
type Budgets struct {
	// MaxAttempts включает первую попытку; для повторов значение должно быть положительным.
	MaxAttempts int

	// Бюджеты ошибок останавливают выполнение на N-й ошибке соответствующего типа.
	// Ноль, как и единица, означает остановку на первой ошибке.
	MaxFastErrors int
	MaxFailures   int
	MaxTimeouts   int

	// Latency ограничивает весь запрос, включая ожидания; значение должно быть положительным.
	Latency time.Duration

	// SubrequestLatency ограничивает одну попытку. Ноль означает остаток общего времени.
	SubrequestLatency time.Duration
}

func (b Budgets) validateTime() error {
	if b.Latency <= 0 || b.SubrequestLatency < 0 {
		return fmt.Errorf("%w: latency must be positive and subrequest latency nonnegative", ErrInvalidConfig)
	}
	return nil
}

func (b Budgets) validateRetries() error {
	if err := b.validateTime(); err != nil {
		return err
	}
	if b.MaxAttempts <= 0 || b.MaxFastErrors < 0 || b.MaxFailures < 0 || b.MaxTimeouts < 0 {
		return fmt.Errorf("%w: attempts must be positive and error budgets nonnegative", ErrInvalidConfig)
	}
	return nil
}

type errorCounts struct {
	fastErrors int
	failures   int
	timeouts   int
}

// record списывает только бюджет ошибки, полученной в этой попытке.
func (c *errorCounts) record(result outcome, budgets Budgets) error {
	switch result {
	case outcomeFastError:
		c.fastErrors++
		if c.fastErrors >= budgets.MaxFastErrors {
			return ErrFastErrorsExhausted
		}
	case outcomeFailure:
		c.failures++
		if c.failures >= budgets.MaxFailures {
			return ErrFailuresExhausted
		}
	case outcomeTimeout:
		c.timeouts++
		if c.timeouts >= budgets.MaxTimeouts {
			return ErrTimeoutsExhausted
		}
	}
	return nil
}
