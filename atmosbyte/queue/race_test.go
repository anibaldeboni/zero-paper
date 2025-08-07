package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestCircuitBreakerRaceCondition tests race conditions in CircuitBreaker specifically
func TestCircuitBreakerRaceCondition(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// Simula múltiplas goroutines tentando acessar o circuit breaker simultaneamente
	var wg sync.WaitGroup
	numGoroutines := 100
	numCallsPerGoroutine := 50

	// Força o circuit breaker a abrir
	cb.Call(func() error { return errors.New("error 1") })
	cb.Call(func() error { return errors.New("error 2") })

	// Aguarda o timeout para permitir half-open
	time.Sleep(150 * time.Millisecond)

	// Executa chamadas concorrentes que devem exercitar a transição de estados
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numCallsPerGoroutine; j++ {
				// Alterna entre sucesso e falha para exercitar todas as transições
				if j%3 == 0 {
					cb.Call(func() error { return nil }) // sucesso
				} else {
					cb.Call(func() error { return errors.New("test error") }) // falha
				}
			}
		}(i)
	}

	wg.Wait()

	// Se chegamos aqui sem data race, o teste passou
	t.Logf("Circuit breaker final state: %v", cb.State())
}

// TestQueueConcurrentAccess testa acesso concorrente à queue
func TestQueueConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := WorkerFunc[string](func(ctx context.Context, msg Message[string]) error {
		// Simula processamento rápido
		time.Sleep(1 * time.Millisecond)
		return nil
	})

	config := QueueConfig{
		Workers:           5,
		BufferSize:        100,
		ProcessingTimeout: 1 * time.Second,
		RetryPolicy:       DefaultRetryPolicy(),
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          100 * time.Millisecond,
		},
	}

	queue := NewQueue(ctx, worker, config)

	// Inicia a queue em background
	go func() {
		queue.Start()
	}()

	// Aguarda a queue iniciar
	time.Sleep(10 * time.Millisecond)

	// Executa enqueues concorrentes
	var wg sync.WaitGroup
	numProducers := 20
	messagesPerProducer := 50

	for range numProducers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range messagesPerProducer {
				err := queue.Enqueue("concurrent message")
				if err != nil && err != ErrQueueClosed && err != ErrQueueFull {
					t.Errorf("Unexpected error enqueueing: %v", err)
				}
			}
		}()
	}

	// Aguarda alguns enqueues acontecerem
	time.Sleep(100 * time.Millisecond)

	// Cancela o contexto para iniciar shutdown
	cancel()

	// Aguarda todos os producers terminarem
	wg.Wait()

	// Se chegamos aqui sem data race, o teste passou
	t.Log("Concurrent access test completed successfully")
}

// TestStatsRaceCondition testa acesso concorrente às estatísticas
func TestStatsRaceCondition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := WorkerFunc[string](func(ctx context.Context, msg Message[string]) error {
		time.Sleep(5 * time.Millisecond) // Simula processamento
		return nil
	})

	config := QueueConfig{
		Workers:           3,
		BufferSize:        50,
		ProcessingTimeout: 1 * time.Second,
		RetryPolicy:       DefaultRetryPolicy(),
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          100 * time.Millisecond,
		},
	}

	queue := NewQueue(ctx, worker, config)

	// Inicia a queue
	go func() {
		queue.Start()
	}()

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup

	// Goroutine que enfileira mensagens
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			queue.Enqueue("test message")
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutines que leem estatísticas concorrentemente
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				stats := queue.Stats()
				t.Logf("Stats: %+v", stats) // Log stats for basic validation
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	// Aguarda um pouco e depois cancela
	time.Sleep(200 * time.Millisecond)
	cancel()

	wg.Wait()

	t.Log("Stats race condition test completed successfully")
}
