package queue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ProcessingContext encapsula o contexto de processamento
type ProcessingContext[T any] struct {
	Queue    *Queue[T]
	Message  Message[T]
	WorkerID int
}

// NewProcessingContext cria um novo contexto de processamento
func NewProcessingContext[T any](q *Queue[T], msg Message[T], workerID int) *ProcessingContext[T] {
	return &ProcessingContext[T]{
		Queue:    q,
		Message:  msg,
		WorkerID: workerID,
	}
}

// IsShutdown verifica se a queue está em estado de shutdown
func (pc *ProcessingContext[T]) IsShutdown() bool {
	return pc.Queue.IsShutdown()
}

// GetProcessingContext cria contexto de execução com timeout adequado
func (pc *ProcessingContext[T]) GetProcessingContext() (context.Context, context.CancelFunc) {
	if pc.IsShutdown() {
		return context.WithTimeout(context.Background(), pc.Queue.config.ProcessingTimeout)
	}
	return pc.Queue.ctx, func() {}
}

// ShouldRetry verifica se deve retentar usando a lógica centralizada
func (pc *ProcessingContext[T]) ShouldRetry(err error) bool {
	return pc.Message.Attempts < pc.Message.MaxTries &&
		ShouldRetry(pc.Queue.ctx, err, pc.Message.Attempts, pc.Message.MaxTries) &&
		err != ErrCircuitBreakerOpen
}

// LogError registra erro com prefixo adequado
func (pc *ProcessingContext[T]) LogError(err error) {
	prefix := pc.getLogPrefix()
	log.Printf("%sWorker %d: Error processing message %s (attempt %d/%d): %v",
		prefix, pc.WorkerID, pc.Message.ID,
		pc.Message.Attempts, pc.Message.MaxTries, err)
}

// LogSuccess registra sucesso com prefixo adequado
func (pc *ProcessingContext[T]) LogSuccess() {
	prefix := pc.getLogPrefix()
	log.Printf("%sWorker %d: Successfully processed message %s",
		prefix, pc.WorkerID, pc.Message.ID)
}

// LogRetryAction registra ação de retry
func (pc *ProcessingContext[T]) LogRetryAction() {
	if pc.IsShutdown() {
		prefix := pc.getLogPrefix()
		log.Printf("%sWorker %d: Queueing message %s for immediate retry (shutdown mode)",
			prefix, pc.WorkerID, pc.Message.ID)
	}
}

// LogDrop registra quando mensagem é descartada
func (pc *ProcessingContext[T]) LogDrop(reason string) {
	prefix := pc.getLogPrefix()
	log.Printf("%sWorker %d: %s",
		prefix, pc.WorkerID, reason)
}

// getLogPrefix retorna o prefixo adequado para logs
func (pc *ProcessingContext[T]) getLogPrefix() string {
	if pc.IsShutdown() {
		return "[SHUTDOWN] "
	}
	return ""
}

// Queue representa uma fila de processamento de mensagens
type Queue[T any] struct {
	config         QueueConfig
	messagesQueue  chan Message[T]
	retryQueue     chan Message[T]
	worker         Worker[T]
	circuitBreaker *CircuitBreaker
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewQueue cria uma nova instância da fila (não inicia os workers)
func NewQueue[T any](ctx context.Context, worker Worker[T], config QueueConfig) *Queue[T] {
	queueCtx, cancel := context.WithCancel(ctx)

	q := &Queue[T]{
		config:        config,
		messagesQueue: make(chan Message[T], config.BufferSize),
		retryQueue:    make(chan Message[T], config.BufferSize),
		worker:        worker,
		circuitBreaker: NewCircuitBreaker(
			config.CircuitBreakerConfig.FailureThreshold,
			config.CircuitBreakerConfig.Timeout,
		),
		ctx:    queueCtx,
		cancel: cancel,
	}

	return q
}

// IsShutdown verifica se a queue está em estado de shutdown
func (q *Queue[T]) IsShutdown() bool {
	return q.ctx.Err() != nil
}

// Start inicia todos os workers da queue e bloqueia até o contexto ser cancelado
func (q *Queue[T]) Start() error {
	log.Printf("Starting queue with %d workers", q.config.Workers)

	for i := 0; i < q.config.Workers; i++ {
		q.wg.Add(1)
		go q.workerLoop(i)
	}

	q.wg.Add(1)
	go q.retryLoop()

	<-q.ctx.Done()
	q.wg.Wait()

	return q.ctx.Err()
}

// Enqueue adiciona uma nova mensagem à fila
func (q *Queue[T]) Enqueue(data T) error {
	if q.IsShutdown() {
		return ErrQueueClosed
	}

	msg := Message[T]{
		ID:        generateID(),
		Data:      data,
		Attempts:  0,
		MaxTries:  q.config.RetryPolicy.MaxRetries,
		CreatedAt: time.Now(),
	}

	select {
	case <-q.ctx.Done():
		return ErrQueueClosed
	case q.messagesQueue <- msg:
		return nil
	default:
		return ErrQueueFull
	}
}

// workerLoop é o loop principal de processamento de cada worker
func (q *Queue[T]) workerLoop(workerID int) {
	defer q.wg.Done()

	for {
		select {
		case msg, ok := <-q.messagesQueue:
			if !ok {
				log.Printf("Worker %d: Messages channel closed, shutting down", workerID)
				return
			}
			q.processMessage(msg, workerID)
		case <-q.ctx.Done():
			q.drainMessageQueue(workerID)
			return
		}
	}
}

func (q *Queue[T]) drainMessageQueue(workerID int) {
	for {
		select {
		case msg, ok := <-q.messagesQueue:
			if !ok {
				log.Printf("Worker %d: Messages channel closed during shutdown", workerID)
				return
			}
			log.Printf("Worker %d: Dropping message %s due to shutdown", workerID, msg.ID)
		default:
			log.Printf("Worker %d: No more messages to process, shutting down", workerID)
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
				log.Printf("RetryLoop: Retry channel closed, shutting down")
				return
			}
			q.handleRetryMessage(msg)

		case <-q.ctx.Done():
			q.drainRetryQueue()
			return
		}
	}
}

// drainRetryQueue drena mensagens restantes durante shutdown
func (q *Queue[T]) drainRetryQueue() {
	for {
		select {
		case msg, ok := <-q.retryQueue:
			if !ok {
				log.Printf("RetryLoop: Retry channel closed during shutdown")
				return
			}
			log.Printf("RetryLoop: Dropping retry message %s due to shutdown", msg.ID)
		default:
			log.Printf("RetryLoop: No more retry messages, shutting down")
			return
		}
	}
}

// handleRetryMessage processa uma mensagem de retry durante operação normal
func (q *Queue[T]) handleRetryMessage(msg Message[T]) {
	delay := q.config.RetryPolicy.CalculateDelay(msg.Attempts)
	log.Printf("RetryLoop: Waiting %v before retrying message %s", delay, msg.ID)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		q.sendToMainQueue(msg)
	case <-q.ctx.Done():
		log.Printf("RetryLoop: Dropping message %s due to context cancellation during delay", msg.ID)
	}
}

// sendToMainQueue tenta enviar mensagem para fila principal
func (q *Queue[T]) sendToMainQueue(msg Message[T]) {
	select {
	case q.messagesQueue <- msg:
		// Mensagem enviada com sucesso
	case <-q.ctx.Done():
		log.Printf("RetryLoop: Dropping message %s due to context cancellation", msg.ID)
	default:
		log.Printf("RetryLoop: Messages queue full, dropping message %s", msg.ID)
	}
}

// processMessage processa uma mensagem individual usando strategy pattern
func (q *Queue[T]) processMessage(msg Message[T], workerID int) {
	msg.Attempts++
	msg.LastTry = time.Now()

	pc := NewProcessingContext(q, msg, workerID)
	result := q.executeProcessing(pc)

	q.handleProcessingResult(pc, result)
}

// ProcessingResult encapsula o resultado do processamento
type ProcessingResult struct {
	Error   error
	Success bool
}

// executeProcessing executa o processamento da mensagem
func (q *Queue[T]) executeProcessing(pc *ProcessingContext[T]) ProcessingResult {
	ctx, cancel := pc.GetProcessingContext()
	defer cancel()

	err := q.circuitBreaker.Call(func() error {
		return q.worker.Process(ctx, pc.Message)
	})

	return ProcessingResult{
		Error:   err,
		Success: err == nil,
	}
}

// handleProcessingResult trata o resultado do processamento
func (q *Queue[T]) handleProcessingResult(pc *ProcessingContext[T], result ProcessingResult) {
	if result.Success {
		pc.LogSuccess()
		return
	}

	pc.LogError(result.Error)

	if q.IsShutdown() {
		pc.LogDrop(fmt.Sprintf("Dropping message %s during shutdown (attempt %d/%d)",
			pc.Message.ID, pc.Message.Attempts, pc.Message.MaxTries))
		return
	}

	if pc.ShouldRetry(result.Error) {
		q.handleRetryAttempt(pc)
	} else {
		pc.LogDrop(fmt.Sprintf("Dropping message %s after %d attempts",
			pc.Message.ID, pc.Message.Attempts))
	}
}

// handleRetryAttempt tenta recolocar mensagem na fila de retry (apenas durante operação normal)
func (q *Queue[T]) handleRetryAttempt(pc *ProcessingContext[T]) {
	pc.LogRetryAction()

	select {
	case q.retryQueue <- pc.Message:
		// Mensagem enviada para retry
	case <-q.ctx.Done():
		pc.LogDrop("Retry queue closed, dropping message")
	default:
		pc.LogDrop("Retry queue full, dropping message")
	}
}

// Stats retorna estatísticas da fila
func (q *Queue[T]) Stats() QueueStats {
	return QueueStats{
		QueueSize:           len(q.messagesQueue),
		RetryQueueSize:      len(q.retryQueue),
		CircuitBreakerState: q.circuitBreaker.State(),
		Workers:             q.config.Workers,
	}
}
