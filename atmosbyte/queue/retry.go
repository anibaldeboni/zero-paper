package queue

import (
	"context"
	"time"
)

// RetryPolicy define a política de retry
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy retorna uma política de retry padrão
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}
}

// CalculateDelay calcula o delay para a próxima tentativa usando backoff exponential
func (rp RetryPolicy) CalculateDelay(attempt int) time.Duration {
	return min(rp.BaseDelay*time.Duration(1<<uint(attempt)), rp.MaxDelay)
}

// ShouldRetry determina se um erro deve ser retentado
// Detecta automaticamente se está em shutdown através do contexto
func ShouldRetry(ctx context.Context, err error, attempts int, maxTries int) bool {
	// Verifica se o erro implementa RetryableError
	retryableErr, isRetryableType := err.(RetryableError)

	// Detecta se estamos em processo de shutdown
	isShutdown := ctx.Err() != nil

	// Durante shutdown, aplicamos regras específicas
	if isShutdown {
		// Se não é retentável por natureza, não tenta durante shutdown
		if isRetryableType && !retryableErr.IsRetryable() {
			return false
		}

		// Durante shutdown, apenas uma tentativa adicional é permitida
		// independentemente do maxTries original
		return attempts < 2 && err != ErrCircuitBreakerOpen
	}

	// Comportamento normal (não shutdown)
	if isRetryableType {
		return retryableErr.IsRetryable()
	}

	// Fallback: outros tipos de erro são considerados retentáveis por padrão
	// Como timeout, connection refused, etc.
	return true
}
