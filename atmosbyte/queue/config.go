package queue

import "time"

// QueueConfig define a configuração da fila
type QueueConfig struct {
	Workers              int
	BufferSize           int
	RetryPolicy          RetryPolicy
	CircuitBreakerConfig CircuitBreakerConfig
}

// CircuitBreakerConfig define a configuração do circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int
	Timeout          time.Duration
}

// DefaultQueueConfig retorna uma configuração padrão para a fila
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		Workers:     3,
		BufferSize:  100,
		RetryPolicy: DefaultRetryPolicy(),
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          30 * time.Second,
		},
	}
}

// QueueStats representa estatísticas da fila
type QueueStats struct {
	QueueSize           int
	RetryQueueSize      int
	CircuitBreakerState CircuitBreakerState
	Workers             int
}
