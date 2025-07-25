package queue

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// OrderData exemplo de tipo customizado
type OrderData struct {
	ID       string  `json:"id"`
	Amount   float64 `json:"amount"`
	Customer string  `json:"customer"`
}

// OrderWorker exemplo de worker para processar pedidos
type OrderWorker struct {
	processedOrders []OrderData
}

func (w *OrderWorker) Process(ctx context.Context, msg Message[OrderData]) error {
	// Simula falha para pedidos com valor muito alto
	if msg.Data.Amount > 1000 {
		return NewHTTPError(500, "Payment service unavailable")
	}

	// Só adiciona ao slice se o processamento foi bem-sucedido
	w.processedOrders = append(w.processedOrders, msg.Data)
	return nil
}

func TestGenericQueue_CustomType(t *testing.T) {
	worker := &OrderWorker{}
	config := DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 2
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	// Cria uma fila genérica para OrderData
	q := NewQueue[OrderData](worker, config)
	defer q.Close()

	// Testa com pedido normal
	order1 := OrderData{
		ID:       "ORD001",
		Amount:   99.50,
		Customer: "João Silva",
	}

	err := q.Enqueue(order1)
	if err != nil {
		t.Fatalf("Failed to enqueue order: %v", err)
	}

	// Testa com pedido que vai falhar e ser retentado
	order2 := OrderData{
		ID:       "ORD002",
		Amount:   1500.00, // Vai falhar
		Customer: "Maria Santos",
	}

	err = q.Enqueue(order2)
	if err != nil {
		t.Fatalf("Failed to enqueue order: %v", err)
	}

	// Aguarda processamento e retries
	time.Sleep(500 * time.Millisecond)

	// Verifica se apenas o primeiro pedido foi processado com sucesso
	if len(worker.processedOrders) != 1 {
		t.Errorf("Expected 1 processed order, got %d. Orders: %+v", len(worker.processedOrders), worker.processedOrders)
	}

	if len(worker.processedOrders) > 0 && worker.processedOrders[0].ID != "ORD001" {
		t.Errorf("Expected ORD001, got %s", worker.processedOrders[0].ID)
	}
}

// OrderWorkerWithRetry é um worker que simula falhas nas duas primeiras tentativas
type OrderWorkerWithRetry struct {
	attemptCounts   *map[string]int
	processedOrders *[]OrderData
}

func (w *OrderWorkerWithRetry) Process(ctx context.Context, msg Message[OrderData]) error {
	orderID := msg.Data.ID

	// Incrementa contador de tentativas
	(*w.attemptCounts)[orderID]++

	// Falha nas duas primeiras tentativas
	if (*w.attemptCounts)[orderID] < 3 {
		return NewHTTPError(500, "Temporary service unavailable")
	}

	// Sucesso na terceira tentativa
	*w.processedOrders = append(*w.processedOrders, msg.Data)
	return nil
}

// TestGenericQueue_RetryBehavior testa especificamente o comportamento de retry
func TestGenericQueue_RetryBehavior(t *testing.T) {
	attemptCounts := make(map[string]int)
	var processedOrders []OrderData

	// Worker que conta tentativas e só processa na terceira tentativa
	worker := &OrderWorkerWithRetry{
		attemptCounts:   &attemptCounts,
		processedOrders: &processedOrders,
	}

	config := DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10
	config.RetryPolicy.MaxRetries = 3
	config.RetryPolicy.BaseDelay = 10 * time.Millisecond

	q := NewQueue[OrderData](worker, config)
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

	// Verifica se foram feitas 3 tentativas
	if attemptCounts["ORD_RETRY"] != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCounts["ORD_RETRY"])
	}

	// Verifica se o pedido foi processado com sucesso na terceira tentativa
	if len(processedOrders) != 1 {
		t.Errorf("Expected 1 processed order, got %d", len(processedOrders))
	}
}

// EventData exemplo de outro tipo customizado
type EventData struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// EventWorker exemplo de worker para processar eventos
type EventWorker struct {
	events []EventData
}

func (w *EventWorker) Process(ctx context.Context, msg Message[EventData]) error {
	w.events = append(w.events, msg.Data)
	fmt.Printf("Processing event: %s at %v\n", msg.Data.Type, msg.Data.Timestamp)
	return nil
}

func TestGenericQueue_EventData(t *testing.T) {
	worker := &EventWorker{}
	config := DefaultQueueConfig()
	config.Workers = 2
	config.BufferSize = 20

	// Cria uma fila genérica para EventData
	q := NewQueue[EventData](worker, config)
	defer q.Close()

	// Envia vários eventos
	events := []EventData{
		{
			Type:      "user.login",
			Timestamp: time.Now(),
			Payload:   map[string]interface{}{"user_id": "123", "ip": "192.168.1.1"},
		},
		{
			Type:      "order.created",
			Timestamp: time.Now(),
			Payload:   map[string]interface{}{"order_id": "ORD001", "amount": 99.50},
		},
		{
			Type:      "email.sent",
			Timestamp: time.Now(),
			Payload:   map[string]interface{}{"to": "user@example.com", "template": "welcome"},
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

	if len(worker.events) != 3 {
		t.Errorf("Expected 3 processed events, got %d", len(worker.events))
	}
}

// Demonstra como usar WorkerFunc com tipos genéricos
func TestGenericQueue_WorkerFunc(t *testing.T) {
	var processedMessages []string

	// Cria um worker usando WorkerFunc
	workerFunc := WorkerFunc[string](func(ctx context.Context, msg Message[string]) error {
		processedMessages = append(processedMessages, msg.Data)
		return nil
	})

	config := DefaultQueueConfig()
	config.Workers = 1
	config.BufferSize = 10

	// Cria uma fila genérica para strings
	q := NewQueue[string](workerFunc, config)
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
