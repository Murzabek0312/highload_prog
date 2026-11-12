package resilience

import (
	"context"
	"fmt"
	"time"
)

type outcome uint8

const (
	outcomeFinal outcome = iota // Успех, внешняя отмена или ошибка, не допускающая повтор.
	outcomeFastError
	outcomeFailure
	outcomeTimeout
)

func sendAttempt(ctx context.Context, sender Sender, timeout time.Duration) (Response, outcome, error) {
	if err := contextError(ctx); err != nil {
		return Response{}, outcomeFinal, err
	}

	attemptCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		attemptCtx, cancel = context.WithTimeoutCause(ctx, timeout, ErrSubrequestTimeout)
		defer cancel()
	}

	response, err := sender.Send(attemptCtx)
	// Общий deadline и внешняя отмена завершают запрос без повторных попыток.
	if parentErr := contextError(ctx); parentErr != nil {
		return Response{}, outcomeFinal, parentErr
	}
	if attemptErr := contextError(attemptCtx); attemptErr != nil {
		return Response{}, outcomeTimeout, attemptErr
	}
	if err != nil {
		return Response{}, outcomeFinal, fmt.Errorf("send: %w", err)
	}

	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		return response, outcomeFinal, nil
	case response.StatusCode >= 400 && response.StatusCode < 500:
		return response, outcomeFastError, &StatusError{StatusCode: response.StatusCode}
	case response.StatusCode >= 500 && response.StatusCode < 600:
		return response, outcomeFailure, &StatusError{StatusCode: response.StatusCode}
	default:
		return response, outcomeFinal, fmt.Errorf("%w: %d", ErrInvalidResponse, response.StatusCode)
	}
}
