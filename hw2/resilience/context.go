package resilience

import (
	"context"
	"errors"
	"time"
)

// contextError проверяет deadline синхронно: новую попытку нельзя запускать
// после его истечения, даже если обработчик отмены еще не выполнился.
func contextError(ctx context.Context) error {
	if ctx.Err() == nil {
		if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			// Ждем, пока обработчик отмены установит соответствующую причину.
			<-ctx.Done()
		}
	}

	err := ctx.Err()
	if err == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != err {
		return errors.Join(err, cause)
	}
	return err
}

// fitsBeforeDeadline сообщает, останется ли время на попытку после ожидания duration.
func fitsBeforeDeadline(ctx context.Context, duration time.Duration) bool {
	deadline, ok := ctx.Deadline()
	return !ok || duration < time.Until(deadline)
}

func wait(ctx context.Context, duration time.Duration) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if duration <= 0 {
		return nil
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return contextError(ctx)
	case <-timer.C:
		return contextError(ctx)
	}
}
