package queue

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockWorker é um worker de teste
type MockWorker struct {
	processFunc func(ctx context.Context, msg MeasurementMessage) error
	calls       []MeasurementMessage
}

func (m *MockWorker) Process(ctx context.Context, msg MeasurementMessage) error {
	m.calls = append(m.calls, msg)
	if m.processFunc != nil {
		return m.processFunc(ctx, msg)
	}
	return nil
}

func TestQueue_Basic(t *testing.T) {
	worker := &MockWorker{}
	config := DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	q := NewMeasurementQueue(worker, config)
	defer q.Close()

	// Testa enqueue básico
	measurement := Measurement{
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    1013,
	}

	err := q.Enqueue(measurement)
	if err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Aguarda processamento
	time.Sleep(100 * time.Millisecond)

	if len(worker.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(worker.calls))
	}

	if worker.calls[0].Data.Temperature != 25.5 {
		t.Errorf("Expected temperature 25.5, got %f", worker.calls[0].Data.Temperature)
	}
}

func TestQueue_Retry(t *testing.T) {
	callCount := 0
	worker := &MockWorker{
		processFunc: func(ctx context.Context, msg MeasurementMessage) error {
			callCount++
			if callCount < 3 {
				// Simula erro HTTP 500 (deve ser retentado)
				return NewHTTPError(500, "Internal Server Error")
			}
			return nil
		},
	}

	config := DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	q := NewMeasurementQueue(worker, config)
	defer q.Close()

	measurement := Measurement{
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    1013,
	}

	err := q.Enqueue(measurement)
	if err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Aguarda processamento e retries
	time.Sleep(500 * time.Millisecond)

	if callCount != 3 {
		t.Errorf("Expected 3 calls (2 retries), got %d", callCount)
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// Testa estado inicial (fechado)
	if cb.State() != CircuitBreakerClosed {
		t.Errorf("Expected closed state, got %v", cb.State())
	}

	// Simula falhas
	err := cb.Call(func() error {
		return errors.New("test error")
	})
	if err == nil {
		t.Error("Expected error")
	}

	err = cb.Call(func() error {
		return errors.New("test error")
	})
	if err == nil {
		t.Error("Expected error")
	}

	// Circuit breaker deve estar aberto agora
	if cb.State() != CircuitBreakerOpen {
		t.Errorf("Expected open state, got %v", cb.State())
	}

	// Tentativa deve ser rejeitada
	err = cb.Call(func() error {
		return nil
	})
	if err != ErrCircuitBreakerOpen {
		t.Errorf("Expected circuit breaker open error, got %v", err)
	}

	// Aguarda timeout
	time.Sleep(150 * time.Millisecond)

	// Deve estar em half-open
	err = cb.Call(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Unexpected error in half-open state: %v", err)
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "HTTP 500 should retry",
			err:      NewHTTPError(500, "Internal Server Error"),
			expected: true,
		},
		{
			name:     "HTTP 503 should retry",
			err:      NewHTTPError(503, "Service Unavailable"),
			expected: true,
		},
		{
			name:     "HTTP 400 should not retry",
			err:      NewHTTPError(400, "Bad Request"),
			expected: false,
		},
		{
			name:     "HTTP 404 should not retry",
			err:      NewHTTPError(404, "Not Found"),
			expected: false,
		},
		{
			name:     "Generic error should retry",
			err:      errors.New("generic error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRetry(tt.err)
			if result != tt.expected {
				t.Errorf("ShouldRetry(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}
