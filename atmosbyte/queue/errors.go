package queue

import "errors"

// Erros customizados
var (
	ErrQueueClosed        = errors.New("queue is closed")
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
)

// RetryableError define a interface para erros que podem ser retentados
type RetryableError interface {
	error
	IsRetryable() bool
}

// SimpleRetryableError implementação simples de RetryableError
type SimpleRetryableError struct {
	err       error
	retryable bool
}

func (e *SimpleRetryableError) Error() string {
	return e.err.Error()
}

func (e *SimpleRetryableError) IsRetryable() bool {
	return e.retryable
}

// NewRetryableError cria um novo erro retentável
func NewRetryableError(err error, retryable bool) *SimpleRetryableError {
	return &SimpleRetryableError{
		err:       err,
		retryable: retryable,
	}
}
