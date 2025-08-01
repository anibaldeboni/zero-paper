package queue_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anibaldeboni/zero-paper/atmosbyte/config"
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

func (w *OrderWorker) GetProcessedOrders() []OrderData {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Retorna uma cópia para evitar race conditions
	result := make([]OrderData, len(w.processedOrders))
	copy(result, w.processedOrders)
	return result
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
	mu          sync.Mutex
}

func (m *MockWorker) Process(ctx context.Context, msg queue.Message[OrderData]) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, msg.Data)
	if m.processFunc != nil {
		return m.processFunc(ctx, msg.Data)
	}
	return nil
}

func (m *MockWorker) GetCalls() []OrderData {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]OrderData, len(m.calls))
	copy(result, m.calls)
	return result
}

// startQueueForTest inicia uma queue em background para testes
func startQueueForTest[T any](t *testing.T, ctx context.Context, worker queue.Worker[T], config queue.QueueConfig) *queue.Queue[T] {
	t.Helper()
	q := queue.NewQueue(ctx, worker, config)

	// Inicia a queue em background
	go func() {
		if err := q.Start(); err != nil && err != context.Canceled {
			t.Logf("Queue error: %v", err)
		}
	}()

	return q
}

// createQueueForTest apenas cria uma queue sem iniciar (para testes que querem controlar o Start())
func createQueueForTest[T any](t *testing.T, ctx context.Context, worker queue.Worker[T], config queue.QueueConfig) *queue.Queue[T] {
	t.Helper()
	return queue.NewQueue(ctx, worker, config)
}

func TestQueue_Basic(t *testing.T) {
	worker := &MockWorker{}
	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := startQueueForTest(t, ctx, worker, config)

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

	calls := worker.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	// Example: check the Amount field instead (OrderData does not have Temperature)
	if calls[0].Amount != 100.0 {
		t.Errorf("Expected amount 100.0, got %f", calls[0].Amount)
	}
}

// ==============================
// Teste do sistema de retry
// ==============================

func TestQueue_Retry(t *testing.T) {
	var callCount int32
	worker := &MockWorker{
		processFunc: func(ctx context.Context, msg OrderData) error {
			count := atomic.AddInt32(&callCount, 1)
			if count < 3 {
				// Simula erro HTTP 500 (deve ser retentado)
				return NewHTTPError(500, "Internal Server Error")
			}
			return nil
		},
	}

	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := startQueueForTest(t, ctx, worker, config)

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

	finalCount := atomic.LoadInt32(&callCount)
	if finalCount != 3 {
		t.Errorf("Expected 3 calls (2 retries), got %d", finalCount)
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
			// Testa comportamento normal (não shutdown)
			ctx := context.Background()
			result := queue.ShouldRetry(ctx, tt.err, 1, 3)
			if result != tt.expected {
				t.Errorf("ShouldRetry(ctx, %v, 1, 3) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestShouldRetryDuringShutdown(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		attempts int
		maxTries int
		expected bool
	}{
		{
			name:     "HTTP 500 during shutdown - first attempt should retry",
			err:      NewHTTPError(500, "Internal Server Error"),
			attempts: 1,
			maxTries: 3,
			expected: true,
		},
		{
			name:     "HTTP 500 during shutdown - second attempt should not retry",
			err:      NewHTTPError(500, "Internal Server Error"),
			attempts: 2,
			maxTries: 3,
			expected: false,
		},
		{
			name:     "HTTP 404 during shutdown should not retry",
			err:      NewHTTPError(404, "Not Found"),
			attempts: 1,
			maxTries: 3,
			expected: false,
		},
		{
			name:     "Circuit breaker open during shutdown should not retry",
			err:      queue.ErrCircuitBreakerOpen,
			attempts: 1,
			maxTries: 3,
			expected: false,
		},
		{
			name:     "Generic error during shutdown - first attempt should retry",
			err:      errors.New("connection timeout"),
			attempts: 1,
			maxTries: 3,
			expected: true,
		},
		{
			name:     "Custom non-retryable error during shutdown should not retry",
			err:      &CustomError{message: "permanent failure", retryable: false},
			attempts: 1,
			maxTries: 3,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cria um contexto cancelado para simular shutdown
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancela imediatamente para simular shutdown

			result := queue.ShouldRetry(ctx, tt.err, tt.attempts, tt.maxTries)
			if result != tt.expected {
				t.Errorf("ShouldRetry(canceledCtx, %v, %d, %d) = %v, expected %v",
					tt.err, tt.attempts, tt.maxTries, result, tt.expected)
			}
		})
	}
}

// ==============================
// Testes de tipos genéricos
// ==============================

func TestGenericQueue_CustomType_Main(t *testing.T) {
	worker := &OrderWorker{}
	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 2
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := startQueueForTest(t, ctx, worker, config)

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
	processedOrders := worker.GetProcessedOrders()
	if len(processedOrders) != 1 {
		t.Errorf("Expected 1 processed order, got %d. Orders: %+v", len(processedOrders), processedOrders)
	}

	if len(processedOrders) > 0 && processedOrders[0].ID != "ORD001" {
		t.Errorf("Expected ORD001, got %s", processedOrders[0].ID)
	}
}

// ==============================
// Teste do WorkerFunc genérico
// ==============================

func TestGenericQueue_WorkerFunc_QueueTest(t *testing.T) {
	var processedMessages []string
	var mu sync.Mutex

	// Cria um worker usando WorkerFunc
	workerFunc := queue.WorkerFunc[string](func(ctx context.Context, msg queue.Message[string]) error {
		mu.Lock()
		defer mu.Unlock()
		processedMessages = append(processedMessages, msg.Data)
		return nil
	})

	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := startQueueForTest(t, ctx, workerFunc, config)

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

	mu.Lock()
	processedCount := len(processedMessages)
	mu.Unlock()

	if processedCount != 5 {
		t.Errorf("Expected 5 processed messages, got %d", processedCount)
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

func (w *RetryCounterWorker) GetProcessedOrders() []OrderData {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Retorna uma cópia para evitar race conditions
	result := make([]OrderData, len(w.processedOrders))
	copy(result, w.processedOrders)
	return result
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

	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := startQueueForTest(t, ctx, worker, config)

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
	worker.mu.Unlock()

	processedOrders := worker.GetProcessedOrders()
	processed := len(processedOrders)

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
	var mu sync.Mutex

	// Worker que processa eventos
	eventWorker := queue.WorkerFunc[EventData](func(ctx context.Context, msg queue.Message[EventData]) error {
		mu.Lock()
		defer mu.Unlock()
		processedEvents = append(processedEvents, msg.Data)
		return nil
	})

	config := config.TestQueueConfig()
	config.Workers = 2
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := startQueueForTest(t, ctx, eventWorker, config)

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

	mu.Lock()
	eventsCopy := make([]EventData, len(processedEvents))
	copy(eventsCopy, processedEvents)
	mu.Unlock()

	if len(eventsCopy) != 3 {
		t.Errorf("Expected 3 processed events, got %d", len(eventsCopy))
	}

	// Verifica se os tipos dos eventos estão corretos
	expectedTypes := []string{"user.login", "order.created", "payment.processed"}
	processedTypes := make([]string, len(eventsCopy))
	for i, event := range eventsCopy {
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

// ==============================
// Testes de Graceful Shutdown
// ==============================

// TrackingWorker rastreia mensagens processadas e permite controle de timing
type TrackingWorker struct {
	processedMessages []string
	processingTimes   []time.Duration
	blockDuration     time.Duration
	shouldBlock       bool
	mu                sync.Mutex
	processStarted    chan struct{}
	allowCompletion   chan struct{}
}

func NewTrackingWorker(blockDuration time.Duration) *TrackingWorker {
	return &TrackingWorker{
		blockDuration:   blockDuration,
		processStarted:  make(chan struct{}, 10),
		allowCompletion: make(chan struct{}, 10),
	}
}

func (w *TrackingWorker) Process(ctx context.Context, msg queue.Message[string]) error {
	start := time.Now()

	w.mu.Lock()
	messageID := msg.Data
	w.mu.Unlock()

	// Sinaliza que o processamento começou
	w.processStarted <- struct{}{}

	// Se deve bloquear, aguarda sinal para continuar
	if w.shouldBlock {
		select {
		case <-w.allowCompletion:
			// Permitido continuar
		case <-ctx.Done():
			// Contexto cancelado durante processamento
			w.mu.Lock()
			w.processedMessages = append(w.processedMessages, messageID+"_CANCELLED")
			w.processingTimes = append(w.processingTimes, time.Since(start))
			w.mu.Unlock()
			return ctx.Err()
		}
	} else if w.blockDuration > 0 {
		// Simula processamento longo
		select {
		case <-time.After(w.blockDuration):
			// Processamento normal
		case <-ctx.Done():
			// Contexto cancelado durante processamento
			w.mu.Lock()
			w.processedMessages = append(w.processedMessages, messageID+"_CANCELLED")
			w.processingTimes = append(w.processingTimes, time.Since(start))
			w.mu.Unlock()
			return ctx.Err()
		}
	}

	w.mu.Lock()
	w.processedMessages = append(w.processedMessages, messageID)
	w.processingTimes = append(w.processingTimes, time.Since(start))
	w.mu.Unlock()

	return nil
}

func (w *TrackingWorker) GetProcessedMessages() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]string, len(w.processedMessages))
	copy(result, w.processedMessages)
	return result
}

func (w *TrackingWorker) GetProcessingTimes() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]time.Duration, len(w.processingTimes))
	copy(result, w.processingTimes)
	return result
}

func (w *TrackingWorker) SetBlocking(shouldBlock bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.shouldBlock = shouldBlock
}

func (w *TrackingWorker) AllowCompletion() {
	w.allowCompletion <- struct{}{}
}

func (w *TrackingWorker) WaitForProcessingStart() {
	<-w.processStarted
}

func TestGracefulShutdown_Basic(t *testing.T) {
	worker := NewTrackingWorker(50 * time.Millisecond)

	config := config.TestQueueConfig()
	config.Workers = 2
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	q := startQueueForTest(t, ctx, worker, config) // Envia algumas mensagens
	messages := []string{"msg1", "msg2", "msg3"}
	for _, msg := range messages {
		err := q.Enqueue(msg)
		if err != nil {
			t.Fatalf("Failed to enqueue message: %v", err)
		}
	}

	// Aguarda um pouco para processamento começar
	time.Sleep(20 * time.Millisecond)

	// Cancela o contexto (inicia shutdown)
	cancel()

	// Aguarda um tempo razoável para shutdown
	time.Sleep(200 * time.Millisecond)

	processedMessages := worker.GetProcessedMessages()

	// Verifica se pelo menos algumas mensagens foram processadas
	if len(processedMessages) == 0 {
		t.Error("Expected at least some messages to be processed during shutdown")
	}

	t.Logf("Processed messages during shutdown: %v", processedMessages)
}

func TestGracefulShutdown_ProcessingInProgress(t *testing.T) {
	worker := NewTrackingWorker(0) // Sem delay automático
	worker.SetBlocking(true)       // Controle manual do bloqueio

	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	q := startQueueForTest(t, ctx, worker, config)

	// Envia uma mensagem que será processada
	err := q.Enqueue("blocking_message")
	if err != nil {
		t.Fatalf("Failed to enqueue message: %v", err)
	}

	// Aguarda o processamento começar
	worker.WaitForProcessingStart()

	// Cancela o contexto enquanto mensagem está sendo processada
	cancel()

	// Aguarda um pouco para o shutdown começar
	time.Sleep(50 * time.Millisecond)

	// Permite que a mensagem complete o processamento
	worker.AllowCompletion()

	// Aguarda mais tempo para o shutdown completar
	time.Sleep(100 * time.Millisecond)

	processedMessages := worker.GetProcessedMessages()

	// A mensagem deve ter sido processada ou cancelada
	if len(processedMessages) != 1 {
		t.Errorf("Expected exactly 1 processed message, got %d: %v", len(processedMessages), processedMessages)
	}

	// Verifica se a mensagem foi processada (não cancelada)
	if len(processedMessages) > 0 && processedMessages[0] != "blocking_message" {
		t.Logf("Message was cancelled during shutdown: %s", processedMessages[0])
	} else {
		t.Logf("Message completed successfully during shutdown: %s", processedMessages[0])
	}
}

func TestGracefulShutdown_NoNewMessages(t *testing.T) {
	worker := NewTrackingWorker(10 * time.Millisecond)

	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	q := startQueueForTest(t, ctx, worker, config)

	// Envia algumas mensagens iniciais
	for i := 0; i < 3; i++ {
		err := q.Enqueue(fmt.Sprintf("initial_msg_%d", i))
		if err != nil {
			t.Fatalf("Failed to enqueue initial message: %v", err)
		}
	}

	// Aguarda processamento começar
	time.Sleep(20 * time.Millisecond)

	// Cancela o contexto
	cancel()

	// Aguarda um pouco para o shutdown processar mensagens pendentes
	time.Sleep(50 * time.Millisecond)

	// Tenta enviar nova mensagem após shutdown - deve falhar
	err := q.Enqueue("should_fail")
	if err == nil {
		t.Error("Expected error when enqueueing after shutdown, but got nil")
	}

	if err != queue.ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}

	// Aguarda mais tempo para shutdown completar
	time.Sleep(100 * time.Millisecond)

	processedMessages := worker.GetProcessedMessages()
	t.Logf("Messages processed before shutdown rejection: %v", processedMessages)

	// Verifica se não há mensagem "should_fail" processada
	for _, msg := range processedMessages {
		if msg == "should_fail" {
			t.Error("Message sent after shutdown was incorrectly processed")
		}
	}
}

func TestGracefulShutdown_RetryBehavior(t *testing.T) {
	retryCount := 0
	worker := &MockWorker{
		processFunc: func(ctx context.Context, msg OrderData) error {
			retryCount++

			// Verifica se estamos em shutdown
			if ctx.Err() != nil {
				t.Logf("Processing message %s during shutdown (attempt %d)", msg.ID, retryCount)

				// Durante shutdown, falha apenas uma vez para testar retry limitado
				if retryCount == 1 {
					return NewHTTPError(500, "Temporary error during shutdown")
				}
				return nil
			}

			// Comportamento normal
			if retryCount < 2 {
				return NewHTTPError(500, "Temporary error")
			}
			return nil
		},
	}

	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	q := startQueueForTest(t, ctx, worker, config)

	// Envia mensagem que falhará e precisará de retry
	measurement := OrderData{
		ID:       "shutdown_retry_test",
		Amount:   100.0,
		Customer: "Test Customer",
	}

	err := q.Enqueue(measurement)
	if err != nil {
		t.Fatalf("Failed to enqueue message: %v", err)
	}

	// Aguarda primeira tentativa falhar
	time.Sleep(30 * time.Millisecond)

	// Cancela contexto durante retry
	cancel()

	// Aguarda processamento durante shutdown
	time.Sleep(200 * time.Millisecond)

	// Verifica quantas tentativas foram feitas
	t.Logf("Total retry attempts during shutdown: %d", retryCount)

	// Durante shutdown, deve ter limitado os retries
	if retryCount > 2 {
		t.Errorf("Expected at most 2 attempts during shutdown, got %d", retryCount)
	}

	calls := worker.GetCalls()
	if len(calls) == 0 {
		t.Error("Expected at least one call to worker")
	}
}

func TestGracefulShutdown_MultipleWorkers(t *testing.T) {
	worker := NewTrackingWorker(30 * time.Millisecond)

	config := config.TestQueueConfig()
	config.Workers = 3
	config.BufferSize = 20

	ctx, cancel := context.WithCancel(context.Background())
	q := startQueueForTest(t, ctx, worker, config)

	// Envia muitas mensagens para multiple workers
	numMessages := 10
	for i := 0; i < numMessages; i++ {
		err := q.Enqueue(fmt.Sprintf("multi_worker_msg_%d", i))
		if err != nil {
			t.Fatalf("Failed to enqueue message %d: %v", i, err)
		}
	}

	// Aguarda processamento começar
	time.Sleep(50 * time.Millisecond)

	// Cancela contexto
	cancel()

	// Aguarda shutdown
	time.Sleep(200 * time.Millisecond)

	processedMessages := worker.GetProcessedMessages()
	processingTimes := worker.GetProcessingTimes()

	t.Logf("Processed %d out of %d messages", len(processedMessages), numMessages)
	t.Logf("Processing times: %v", processingTimes)

	// Deve ter processado pelo menos algumas mensagens
	if len(processedMessages) == 0 {
		t.Error("Expected some messages to be processed by multiple workers")
	}

	// Verifica se não há duplicates (cada mensagem processada apenas uma vez)
	messageSet := make(map[string]bool)
	for _, msg := range processedMessages {
		if messageSet[msg] {
			t.Errorf("Duplicate message processed: %s", msg)
		}
		messageSet[msg] = true
	}
}

func TestGracefulShutdown_QueueStats(t *testing.T) {
	worker := NewTrackingWorker(100 * time.Millisecond) // Processamento lento

	config := config.TestQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	q := startQueueForTest(t, ctx, worker, config)

	// Envia várias mensagens
	for i := 0; i < 5; i++ {
		err := q.Enqueue(fmt.Sprintf("stats_msg_%d", i))
		if err != nil {
			t.Fatalf("Failed to enqueue message: %v", err)
		}
	}

	// Verifica stats antes do shutdown
	statsBefore := q.Stats()
	t.Logf("Stats before shutdown - Queue: %d, Retry: %d, Workers: %d, CB: %v",
		statsBefore.QueueSize, statsBefore.RetryQueueSize, statsBefore.Workers, statsBefore.CircuitBreakerState)

	// Aguarda um pouco
	time.Sleep(50 * time.Millisecond)

	// Verifica stats durante processamento
	statsDuring := q.Stats()
	t.Logf("Stats during processing - Queue: %d, Retry: %d, Workers: %d, CB: %v",
		statsDuring.QueueSize, statsDuring.RetryQueueSize, statsDuring.Workers, statsDuring.CircuitBreakerState)

	// Cancela contexto
	cancel()

	// Aguarda shutdown
	time.Sleep(300 * time.Millisecond)

	// Verifica stats após shutdown
	statsAfter := q.Stats()
	t.Logf("Stats after shutdown - Queue: %d, Retry: %d, Workers: %d, CB: %v",
		statsAfter.QueueSize, statsAfter.RetryQueueSize, statsAfter.Workers, statsAfter.CircuitBreakerState)

	// Stats devem refletir o estado após shutdown
	if statsAfter.Workers != config.Workers {
		t.Errorf("Worker count should remain %d, got %d", config.Workers, statsAfter.Workers)
	}

	processedMessages := worker.GetProcessedMessages()
	t.Logf("Final processed messages: %v", processedMessages)
}

// TestQueue_StartMethod testa o método Start() da queue
func TestQueue_StartMethod(t *testing.T) {
	worker := NewTrackingWorker(30 * time.Millisecond)

	config := config.TestQueueConfig()
	config.Workers = 2
	config.BufferSize = 10

	ctx, cancel := context.WithCancel(context.Background())
	q := createQueueForTest(t, ctx, worker, config)

	// Inicia a queue em uma goroutine
	done := make(chan error, 1)
	go func() {
		done <- q.Start()
	}()

	// Envia algumas mensagens
	for i := 0; i < 5; i++ {
		err := q.Enqueue(fmt.Sprintf("start_test_msg_%d", i))
		if err != nil {
			t.Fatalf("Failed to enqueue message: %v", err)
		}
	}

	// Aguarda um pouco para processamento começar
	time.Sleep(50 * time.Millisecond)

	// Cancela o contexto para iniciar shutdown
	start := time.Now()
	cancel()

	// Aguarda Start() completar
	err := <-done
	duration := time.Since(start)

	t.Logf("Queue.Start() completed in %v with error: %v", duration, err)

	// Deve retornar context.Canceled
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Verifica se algumas mensagens foram processadas
	processedMessages := worker.GetProcessedMessages()
	t.Logf("Processed messages: %v", processedMessages)

	if len(processedMessages) == 0 {
		t.Error("Expected at least some messages to be processed")
	}

	// Verifica se Start() realmente aguardou o shutdown completo
	// (deve ter levado pelo menos o tempo de processamento de algumas mensagens)
	if duration < 10*time.Millisecond {
		t.Errorf("Start() completed too quickly (%v), might not have waited for shutdown", duration)
	}
}
