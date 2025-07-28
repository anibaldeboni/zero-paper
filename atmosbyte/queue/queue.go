package queue

import (
	"context"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"
)

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

// Message representa uma mensagem na fila com metadados de controle
type Message[T any] struct {
	ID        string    `json:"id"`
	Data      T         `json:"data"`
	Attempts  int       `json:"attempts"`
	MaxTries  int       `json:"max_tries"`
	CreatedAt time.Time `json:"created_at"`
	LastTry   time.Time `json:"last_try"`
}

// Worker define a interface para processamento de mensagens
type Worker[T any] interface {
	Process(ctx context.Context, msg Message[T]) error
}

// WorkerFunc é um adapter que permite usar funções como Worker
type WorkerFunc[T any] func(ctx context.Context, msg Message[T]) error

func (f WorkerFunc[T]) Process(ctx context.Context, msg Message[T]) error {
	return f(ctx, msg)
}

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
	delay := min(rp.BaseDelay*time.Duration(1<<uint(attempt)), rp.MaxDelay)
	return delay
}

// ShouldRetry determina se um erro deve ser retentado usando a nova interface
func ShouldRetry(err error) bool {
	// Verifica se o erro implementa RetryableError
	if retryableErr, ok := err.(RetryableError); ok {
		return retryableErr.IsRetryable()
	}

	// Fallback: outros tipos de erro são considerados retentáveis por padrão
	// Como timeout, connection refused, etc.
	return true
}

// CircuitBreakerState representa o estado do circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// CircuitBreaker implementa o padrão Circuit Breaker
type CircuitBreaker struct {
	mu                sync.RWMutex
	state             CircuitBreakerState
	failureCount      int
	lastFailureTime   time.Time
	successCount      int
	failureThreshold  int
	timeout           time.Duration
	halfOpenSuccesses int
	maxHalfOpenTries  int
}

// NewCircuitBreaker cria um novo circuit breaker
func NewCircuitBreaker(failureThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitBreakerClosed,
		failureThreshold: failureThreshold,
		timeout:          timeout,
		maxHalfOpenTries: 3,
	}
}

// Call executa uma função através do circuit breaker
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.allowCall() {
		return ErrCircuitBreakerOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// allowCall verifica se a chamada é permitida
func (cb *CircuitBreaker) allowCall() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitBreakerHalfOpen
			cb.halfOpenSuccesses = 0
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return cb.halfOpenSuccesses < cb.maxHalfOpenTries
	default:
		return false
	}
}

// recordResult registra o resultado da chamada
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.state == CircuitBreakerHalfOpen {
			cb.state = CircuitBreakerOpen
		} else if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitBreakerOpen
		}
	} else {
		cb.failureCount = 0
		if cb.state == CircuitBreakerHalfOpen {
			cb.halfOpenSuccesses++
			if cb.halfOpenSuccesses >= cb.maxHalfOpenTries {
				cb.state = CircuitBreakerClosed
			}
		}
	}
}

// State retorna o estado atual do circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

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

// Queue representa uma fila de processamento de mensagens
type Queue[T any] struct {
	config         QueueConfig
	messages       chan Message[T]
	retryQueue     chan Message[T]
	worker         Worker[T]
	circuitBreaker *CircuitBreaker
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	closed         bool
	mu             sync.RWMutex
}

// NewQueue cria uma nova instância da fila
func NewQueue[T any](worker Worker[T], config QueueConfig) *Queue[T] {
	ctx, cancel := context.WithCancel(context.Background())

	q := &Queue[T]{
		config:     config,
		messages:   make(chan Message[T], config.BufferSize),
		retryQueue: make(chan Message[T], config.BufferSize),
		worker:     worker,
		circuitBreaker: NewCircuitBreaker(
			config.CircuitBreakerConfig.FailureThreshold,
			config.CircuitBreakerConfig.Timeout,
		),
		ctx:    ctx,
		cancel: cancel,
	}

	// Inicia os workers
	for i := 0; i < config.Workers; i++ {
		q.wg.Add(1)
		go q.workerLoop(i)
	}

	// Inicia o processador de retry
	q.wg.Add(1)
	go q.retryLoop()

	return q
}

// Enqueue adiciona uma nova mensagem à fila
func (q *Queue[T]) Enqueue(data T) error {
	q.mu.RLock()
	if q.closed {
		q.mu.RUnlock()
		return ErrQueueClosed
	}
	q.mu.RUnlock()

	msg := Message[T]{
		ID:        generateID(),
		Data:      data,
		Attempts:  0,
		MaxTries:  q.config.RetryPolicy.MaxRetries,
		CreatedAt: time.Now(),
	}

	select {
	case q.messages <- msg:
		return nil
	case <-q.ctx.Done():
		return ErrQueueClosed
	default:
		// Fila cheia, pode optar por bloquear ou retornar erro
		return errors.New("queue is full")
	}
}

// Close fecha a fila e aguarda o processamento das mensagens pendentes
func (q *Queue[T]) Close() error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	q.mu.Unlock()

	close(q.messages)
	close(q.retryQueue)
	q.cancel()
	q.wg.Wait()
	return nil
}

// workerLoop é o loop principal de processamento de cada worker
func (q *Queue[T]) workerLoop(workerID int) {
	defer q.wg.Done()

	for {
		select {
		case msg, ok := <-q.messages:
			if !ok {
				return
			}
			q.processMessage(msg, workerID)
		case <-q.ctx.Done():
			return
		}
	}
}

// retryLoop processa mensagens que falharam e precisam ser retentadas
func (q *Queue[T]) retryLoop() {
	defer q.wg.Done()

	for {
		select {
		case msg, ok := <-q.retryQueue:
			if !ok {
				return
			}

			// Calcula o delay baseado no número de tentativas
			delay := q.config.RetryPolicy.CalculateDelay(msg.Attempts)

			// Aguarda o delay antes de reprocessar
			time.Sleep(delay)

			// Recoloca na fila principal
			select {
			case q.messages <- msg:
			case <-q.ctx.Done():
				return
			}
		case <-q.ctx.Done():
			return
		}
	}
}

// processMessage processa uma mensagem individual
func (q *Queue[T]) processMessage(msg Message[T], workerID int) {
	msg.Attempts++
	msg.LastTry = time.Now()

	// Processa através do circuit breaker
	err := q.circuitBreaker.Call(func() error {
		return q.worker.Process(q.ctx, msg)
	})

	if err != nil {
		log.Printf("Worker %d: Error processing message %s (attempt %d/%d): %v",
			workerID, msg.ID, msg.Attempts, msg.MaxTries, err)

		// Verifica se deve retentativa
		if msg.Attempts < msg.MaxTries && ShouldRetry(err) && err != ErrCircuitBreakerOpen {
			select {
			case q.retryQueue <- msg:
			case <-q.ctx.Done():
				return
			default:
				log.Printf("Worker %d: Retry queue full, dropping message %s", workerID, msg.ID)
			}
		} else {
			log.Printf("Worker %d: Dropping message %s after %d attempts", workerID, msg.ID, msg.Attempts)
		}
	} else {
		log.Printf("Worker %d: Successfully processed message %s", workerID, msg.ID)
	}
}

// generateID gera um ID único para a mensagem
func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// Stats retorna estatísticas da fila
func (q *Queue[T]) Stats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return QueueStats{
		QueueSize:           len(q.messages),
		RetryQueueSize:      len(q.retryQueue),
		CircuitBreakerState: q.circuitBreaker.State(),
		Workers:             q.config.Workers,
	}
}

// QueueStats representa estatísticas da fila
type QueueStats struct {
	QueueSize           int
	RetryQueueSize      int
	CircuitBreakerState CircuitBreakerState
	Workers             int
}
