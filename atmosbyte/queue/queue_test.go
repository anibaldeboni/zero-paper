package queue_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

// ==============================
// Tipos de teste genéricos
// ==============================

// OrderData para testes de tipos customizados
type OrderData struct {
	ID       string  `json:"id"`
	Amount   float64 `json:"amount"`
	Customer string  `json:"customer"`
}

// PaymentError implementa RetryableError
type PaymentError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *PaymentError) Error() string {
	return fmt.Sprintf("Payment Error %s: %s", e.Code, e.Message)
}

func (e *PaymentError) IsRetryable() bool {
	return e.Retryable
}

// HTTPError representa um erro HTTP com código de status (mantido para compatibilidade)
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// IsRetryable implementa RetryableError para HTTPError
func (e *HTTPError) IsRetryable() bool {
	// Erros HTTP 5xx são retentáveis
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// NewHTTPError cria um novo erro HTTP
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// ValidationError não implementa RetryableError (não retentável)
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Validation Error in %s: %s", e.Field, e.Message)
}

// OrderWorker para testes
type OrderWorker struct {
	processedOrders []OrderData
	mu              sync.Mutex
}

func (w *OrderWorker) Process(ctx context.Context, msg queue.Message[OrderData]) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Falha temporária (será retentada)
	if msg.Data.Amount > 1000 {
		return &PaymentError{
			Code:      "PAYMENT_SERVICE_DOWN",
			Message:   "Payment service temporarily unavailable",
			Retryable: true,
		}
	}

	// Falha permanente (não será retentada)
	if msg.Data.Amount < 0 {
		return &ValidationError{
			Field:   "amount",
			Message: "Amount cannot be negative",
		}
	}

	// Sucesso - adiciona à lista de processados
	w.processedOrders = append(w.processedOrders, msg.Data)
	return nil
}

// EventData para testes de eventos
type EventData struct {
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

// ==============================
// Teste dos tipos básicos
// ==============================

// MockWorker é um worker de teste para Measurement
type MockWorker struct {
	processFunc func(ctx context.Context, msg OrderData) error
	calls       []OrderData
}

func (m *MockWorker) Process(ctx context.Context, msg queue.Message[OrderData]) error {
	m.calls = append(m.calls, msg.Data)
	if m.processFunc != nil {
		return m.processFunc(ctx, msg.Data)
	}
	return nil
}

func TestQueue_Basic(t *testing.T) {
	worker := &MockWorker{}
	config := queue.DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	q := queue.NewQueue(context.Background(), worker, config)
	defer q.Close()

	// Testa enqueue básico
	measurement := OrderData{
		ID:       "ORD123",
		Amount:   100.0,
		Customer: "John Doe",
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

	// Example: check the Amount field instead (OrderData does not have Temperature)
	if worker.calls[0].Amount != 100.0 {
		t.Errorf("Expected amount 100.0, got %f", worker.calls[0].Amount)
	}
}

// ==============================
// Teste do sistema de retry
// ==============================

func TestQueue_Retry(t *testing.T) {
	callCount := 0
	worker := &MockWorker{
		processFunc: func(ctx context.Context, msg OrderData) error {
			callCount++
			if callCount < 3 {
				// Simula erro HTTP 500 (deve ser retentado)
				return NewHTTPError(500, "Internal Server Error")
			}
			return nil
		},
	}

	config := queue.DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	q := queue.NewQueue(context.Background(), worker, config)
	defer q.Close()

	measurement := OrderData{
		ID:       "ORD123",
		Amount:   100.0,
		Customer: "John Doe",
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

// ==============================
// Teste do Circuit Breaker
// ==============================

func TestCircuitBreaker(t *testing.T) {
	cb := queue.NewCircuitBreaker(2, 100*time.Millisecond)

	// Testa estado inicial (fechado)
	if cb.State() != queue.CircuitBreakerClosed {
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
	if cb.State() != queue.CircuitBreakerOpen {
		t.Errorf("Expected open state, got %v", cb.State())
	}

	// Tentativa deve ser rejeitada
	err = cb.Call(func() error {
		return nil
	})
	if err != queue.ErrCircuitBreakerOpen {
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

// ==============================
// Teste da nova interface RetryableError
// ==============================

// CustomError implementa RetryableError
type CustomError struct {
	message   string
	retryable bool
}

func (e *CustomError) Error() string {
	return e.message
}

func (e *CustomError) IsRetryable() bool {
	return e.retryable
}

func TestShouldRetry_NewInterface(t *testing.T) {
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
			name:     "Custom retryable error should retry",
			err:      &CustomError{message: "temporary failure", retryable: true},
			expected: true,
		},
		{
			name:     "Custom non-retryable error should not retry",
			err:      &CustomError{message: "permanent failure", retryable: false},
			expected: false,
		},
		{
			name:     "SimpleRetryableError retryable should retry",
			err:      queue.NewRetryableError(errors.New("temp error"), true),
			expected: true,
		},
		{
			name:     "SimpleRetryableError non-retryable should not retry",
			err:      queue.NewRetryableError(errors.New("perm error"), false),
			expected: false,
		},
		{
			name:     "Generic error should retry (fallback)",
			err:      errors.New("generic error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := queue.ShouldRetry(tt.err)
			if result != tt.expected {
				t.Errorf("ShouldRetry(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// ==============================
// Testes de tipos genéricos
// ==============================

func TestGenericQueue_CustomType_Main(t *testing.T) {
	worker := &OrderWorker{}
	config := queue.DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 2
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	// Cria uma fila genérica para OrderData
	q := queue.NewQueue(context.Background(), worker, config)
	defer q.Close()

	// Testa com pedido normal (sucesso)
	order1 := OrderData{
		ID:       "ORD001",
		Amount:   99.50,
		Customer: "João Silva",
	}

	err := q.Enqueue(order1)
	if err != nil {
		t.Fatalf("Failed to enqueue order: %v", err)
	}

	// Testa com pedido que vai falhar temporariamente e ser retentado
	order2 := OrderData{
		ID:       "ORD002",
		Amount:   1500.00, // Vai falhar temporariamente
		Customer: "Maria Santos",
	}

	err = q.Enqueue(order2)
	if err != nil {
		t.Fatalf("Failed to enqueue order: %v", err)
	}

	// Testa com pedido que vai falhar permanentemente (não retentado)
	order3 := OrderData{
		ID:       "ORD003",
		Amount:   -50.00, // Falha permanente
		Customer: "Carlos Pereira",
	}

	err = q.Enqueue(order3)
	if err != nil {
		t.Fatalf("Failed to enqueue order: %v", err)
	}

	// Aguarda processamento e retries
	time.Sleep(300 * time.Millisecond)

	// Verifica se apenas o primeiro pedido foi processado com sucesso
	if len(worker.processedOrders) != 1 {
		t.Errorf("Expected 1 processed order, got %d. Orders: %+v", len(worker.processedOrders), worker.processedOrders)
	}

	if len(worker.processedOrders) > 0 && worker.processedOrders[0].ID != "ORD001" {
		t.Errorf("Expected ORD001, got %s", worker.processedOrders[0].ID)
	}
}

// ==============================
// Teste do WorkerFunc genérico
// ==============================

func TestGenericQueue_WorkerFunc_QueueTest(t *testing.T) {
	var processedMessages []string

	// Cria um worker usando WorkerFunc
	workerFunc := queue.WorkerFunc[string](func(ctx context.Context, msg queue.Message[string]) error {
		processedMessages = append(processedMessages, msg.Data)
		return nil
	})

	config := queue.DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	// Cria uma fila genérica para strings
	q := queue.NewQueue(context.Background(), workerFunc, config)
	defer q.Close()

	// Envia algumas mensagens
	messages := []string{"Hello", "World", "from", "Generic", "Queue"}

	for _, msg := range messages {
		err := q.Enqueue(msg)
		if err != nil {
			t.Fatalf("Failed to enqueue message: %v", err)
		}
	}

	// Aguarda processamento
	time.Sleep(100 * time.Millisecond)

	if len(processedMessages) != 5 {
		t.Errorf("Expected 5 processed messages, got %d", len(processedMessages))
	}

	for i, expected := range messages {
		if i < len(processedMessages) && processedMessages[i] != expected {
			t.Errorf("Expected message %s, got %s", expected, processedMessages[i])
		}
	}
}

// ==============================
// Teste de retry com contador de tentativas
// ==============================

type RetryCounterWorker struct {
	attemptCounts   map[string]int
	processedOrders []OrderData
	mu              sync.Mutex
}

func (w *RetryCounterWorker) Process(ctx context.Context, msg queue.Message[OrderData]) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	orderID := msg.Data.ID

	// Incrementa contador de tentativas
	w.attemptCounts[orderID]++

	// Falha nas duas primeiras tentativas
	if w.attemptCounts[orderID] < 3 {
		return queue.NewRetryableError(
			errors.New("temporary service unavailable"),
			true, // retryable
		)
	}

	// Sucesso na terceira tentativa
	w.processedOrders = append(w.processedOrders, msg.Data)
	return nil
}

func TestGenericQueue_RetryBehavior_Local(t *testing.T) {
	worker := &RetryCounterWorker{
		attemptCounts: make(map[string]int),
	}

	config := queue.DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	q := queue.NewQueue(context.Background(), worker, config)
	defer q.Close()

	order := OrderData{
		ID:       "ORD_RETRY",
		Amount:   99.50,
		Customer: "Test Customer",
	}

	err := q.Enqueue(order)
	if err != nil {
		t.Fatalf("Failed to enqueue order: %v", err)
	}

	// Aguarda todas as tentativas
	time.Sleep(300 * time.Millisecond)

	worker.mu.Lock()
	attempts := worker.attemptCounts["ORD_RETRY"]
	processed := len(worker.processedOrders)
	worker.mu.Unlock()

	// Verifica se foram feitas 3 tentativas
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// Verifica se o pedido foi processado com sucesso na terceira tentativa
	if processed != 1 {
		t.Errorf("Expected 1 processed order, got %d", processed)
	}
}

// ==============================
// Teste com EventData
// ==============================

func TestGenericQueue_EventData_Complete(t *testing.T) {
	var processedEvents []EventData

	// Worker que processa eventos
	eventWorker := queue.WorkerFunc[EventData](func(ctx context.Context, msg queue.Message[EventData]) error {
		processedEvents = append(processedEvents, msg.Data)
		return nil
	})

	config := queue.DefaultQueueConfig()
	config.Workers = 2
	config.BufferSize = 10

	q := queue.NewQueue(context.Background(), eventWorker, config)
	defer q.Close()

	// Envia alguns eventos
	events := []EventData{
		{
			Type:      "user.login",
			Timestamp: time.Now(),
			Payload:   map[string]any{"user_id": "123", "ip": "192.168.1.1"},
		},
		{
			Type:      "order.created",
			Timestamp: time.Now(),
			Payload:   map[string]any{"order_id": "ORD456", "total": 99.99},
		},
		{
			Type:      "payment.processed",
			Timestamp: time.Now(),
			Payload:   map[string]any{"payment_id": "PAY789", "status": "success"},
		},
	}

	for _, event := range events {
		err := q.Enqueue(event)
		if err != nil {
			t.Fatalf("Failed to enqueue event: %v", err)
		}
	}

	// Aguarda processamento
	time.Sleep(100 * time.Millisecond)

	if len(processedEvents) != 3 {
		t.Errorf("Expected 3 processed events, got %d", len(processedEvents))
	}

	// Verifica se os tipos dos eventos estão corretos
	expectedTypes := []string{"user.login", "order.created", "payment.processed"}
	processedTypes := make([]string, len(processedEvents))
	for i, event := range processedEvents {
		processedTypes[i] = event.Type
	}

	// Como o processamento é paralelo, apenas verificamos se todos os tipos estão presentes
	for _, expectedType := range expectedTypes {
		found := false
		for _, processedType := range processedTypes {
			if processedType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event type %s not found in processed events", expectedType)
		}
	}
}
