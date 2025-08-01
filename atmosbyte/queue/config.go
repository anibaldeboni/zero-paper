package queue

import "time"

// QueueConfig define a configuração da fila
type QueueConfig struct {
	Workers              int
	BufferSize           int
	RetryPolicy          RetryPolicy
	CircuitBreakerConfig CircuitBreakerConfig
	ShutdownTimeout      time.Duration // Timeout para shutdown gracioso
	ProcessingTimeout    time.Duration // Timeout para processamento durante shutdown
}

// CircuitBreakerConfig define a configuração do circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int
	Timeout          time.Duration
}

// QueueStats representa estatísticas da fila
type QueueStats struct {
	QueueSize           int
	RetryQueueSize      int
	CircuitBreakerState CircuitBreakerState
	Workers             int
}
