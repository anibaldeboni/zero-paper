package queue

import "time"

// testQueueConfig returns a queue config for testing purposes
func testQueueConfig() QueueConfig {
	return QueueConfig{
		Workers:           2,
		BufferSize:        100,
		ShutdownTimeout:   30 * time.Second,
		ProcessingTimeout: 5 * time.Second,
		RetryPolicy: RetryPolicy{
			MaxRetries: 3,
			BaseDelay:  time.Second,
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          30 * time.Second,
		},
	}
}
