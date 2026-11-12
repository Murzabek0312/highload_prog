package resilience

import (
	"errors"
	"fmt"
)

// Ошибки некорректных параметров и исчерпания бюджетов запроса.
// Для проверки используйте errors.Is: возвращаемая ошибка может содержать несколько причин.
var (
	ErrInvalidConfig       = errors.New("invalid resilience configuration")
	ErrInvalidResponse     = errors.New("unsupported server status")
	ErrAttemptsExhausted   = errors.New("attempt budget exhausted")
	ErrFastErrorsExhausted = errors.New("fast errors budget exhausted")
	ErrFailuresExhausted   = errors.New("failures budget exhausted")
	ErrTimeoutsExhausted   = errors.New("timeouts budget exhausted")
	ErrLatencyExhausted    = errors.New("latency budget exhausted")
	ErrSubrequestTimeout   = errors.New("subrequest timed out")
	ErrAllReplicasOpen     = errors.New("all replicas are excluded by circuit breaker")
)

// StatusError сохраняет статус последнего неуспешного ответа сервера в цепочке ошибок.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("server returned %d", e.StatusCode)
}
