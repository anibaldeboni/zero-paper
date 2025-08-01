package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ProcessingContext encapsula o contexto de processamento
type ProcessingContext[T any] struct {
	Queue      *Queue[T]
	Message    Message[T]
	WorkerID   int
	IsShutdown bool
}

// NewProcessingContext cria um novo contexto de processamento
func NewProcessingContext[T any](q *Queue[T], msg Message[T], workerID int) *ProcessingContext[T] {
	isShutdown := q.ctx.Err() != nil

	return &ProcessingContext[T]{
		Queue:      q,
		Message:    msg,
		WorkerID:   workerID,
		IsShutdown: isShutdown,
	}
}

// GetProcessingContext cria contexto de execução com timeout adequado
func (pc *ProcessingContext[T]) GetProcessingContext() (context.Context, context.CancelFunc) {
	if pc.IsShutdown {
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
	if pc.IsShutdown {
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
	if pc.IsShutdown {
		return "[SHUTDOWN] "
	}
	return ""
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
	shutdownOnce   sync.Once    // Garante que shutdown só aconteça uma vez
	shutdownFlag   int64        // Flag atômica para indicar shutdown (0 = normal, 1 = shutdown)
	retryMutex     sync.RWMutex // Protege acesso ao retryQueue durante shutdown
}

// NewQueue cria uma nova instância da fila (não inicia os workers)
func NewQueue[T any](ctx context.Context, worker Worker[T], config QueueConfig) *Queue[T] {
	queueCtx, cancel := context.WithCancel(ctx)

	q := &Queue[T]{
		config:     config,
		messages:   make(chan Message[T], config.BufferSize),
		retryQueue: make(chan Message[T], config.BufferSize),
		worker:     worker,
		circuitBreaker: NewCircuitBreaker(
			config.CircuitBreakerConfig.FailureThreshold,
			config.CircuitBreakerConfig.Timeout,
		),
		ctx:          queueCtx,
		cancel:       cancel,
		shutdownFlag: 0, // Inicializa como não shutdown
	}

	return q
}

// Start inicia todos os workers da queue e bloqueia até o contexto ser cancelado
func (q *Queue[T]) Start() error {
	log.Printf("Starting queue with %d workers", q.config.Workers)

	// Inicia os workers
	for i := 0; i < q.config.Workers; i++ {
		q.wg.Add(1)
		go q.workerLoop(i)
	}

	// Inicia o processador de retry
	q.wg.Add(1)
	go q.retryLoop()

	// Inicia o monitor de graceful shutdown
	q.wg.Add(1)
	go q.gracefulShutdownMonitor()

	// Aguarda o contexto ser cancelado
	<-q.ctx.Done()

	// Aguarda todos os workers terminarem graciosamente
	q.wg.Wait()

	log.Println("Queue shutdown completed")
	return q.ctx.Err()
}

// Enqueue adiciona uma nova mensagem à fila
func (q *Queue[T]) Enqueue(data T) error {
	// Verifica se o contexto foi cancelado
	select {
	case <-q.ctx.Done():
		return ErrQueueClosed
	default:
	}

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
		log.Println("Queue context cancelled, cannot enqueue message")
		return ErrQueueClosed
	default:
		// Fila cheia, pode optar por bloquear ou retornar erro
		return errors.New("queue is full")
	}
}

// gracefulShutdownMonitor monitora o contexto pai e executa shutdown gracioso
func (q *Queue[T]) gracefulShutdownMonitor() {
	defer q.wg.Done()

	// Aguarda o contexto pai ser cancelado
	<-q.ctx.Done()

	log.Println("Queue: Starting graceful shutdown...")

	// Sinaliza que shutdown foi iniciado usando sync.Once (thread-safe)
	q.shutdownOnce.Do(func() {
		atomic.StoreInt64(&q.shutdownFlag, 1)
		// Protege o fechamento do retryQueue
		q.retryMutex.Lock()
		defer q.retryMutex.Unlock()
	})

	// Para de aceitar novas mensagens fechando o canal de entrada
	close(q.messages)

	// Aguarda um timeout para que workers processem mensagens pendentes
	shutdownTimeout := q.config.ShutdownTimeout
	shutdownTimer := time.NewTimer(shutdownTimeout)
	defer shutdownTimer.Stop()

	select {
	case <-shutdownTimer.C:
		log.Println("Queue: Shutdown timeout reached, forcing closure")
	case <-time.After(1 * time.Second):
		// Pequeno delay para permitir que workers processem mensagens restantes
	}

	// Fecha o canal de retry após delay, protegido por mutex
	q.retryMutex.Lock()
	close(q.retryQueue)
	q.retryMutex.Unlock()

	log.Println("Queue: Graceful shutdown completed")
}

// isShuttingDown verifica se shutdown foi iniciado de forma thread-safe
func (q *Queue[T]) isShuttingDown() bool {
	return atomic.LoadInt64(&q.shutdownFlag) == 1
}

// workerLoop é o loop principal de processamento de cada worker
func (q *Queue[T]) workerLoop(workerID int) {
	defer q.wg.Done()

	for {
		select {
		case msg, ok := <-q.messages:
			if !ok {
				// Canal fechado, processo de shutdown iniciado
				log.Printf("Worker %d: Messages channel closed, shutting down", workerID)
				return
			}
			q.processMessage(msg, workerID)
		case <-q.ctx.Done():
			// Contexto cancelado - drena mensagens restantes até canal fechar
			for {
				select {
				case msg, ok := <-q.messages:
					if !ok {
						log.Printf("Worker %d: Messages channel closed, shutting down", workerID)
						return
					}
					q.processMessage(msg, workerID)
				default:
					// Não há mais mensagens, pode sair
					log.Printf("Worker %d: Context cancelled, no more messages to process", workerID)
					return
				}
			}
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
				log.Println("RetryLoop: Retry channel closed, shutting down")
				return
			}
			q.handleRetryMessage(msg)

		case <-q.ctx.Done():
			q.handleShutdownRetries()
			return
		}
	}
}

// handleRetryMessage processa uma mensagem de retry usando strategy pattern
func (q *Queue[T]) handleRetryMessage(msg Message[T]) {
	isShutdown := q.ctx.Err() != nil

	if isShutdown {
		q.handleShutdownRetry(msg)
	} else {
		q.handleNormalRetry(msg)
	}
}

// handleNormalRetry processa retry com delay normal
func (q *Queue[T]) handleNormalRetry(msg Message[T]) {
	delay := q.config.RetryPolicy.CalculateDelay(msg.Attempts)
	log.Printf("RetryLoop: Waiting %v before retrying message %s", delay, msg.ID)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		q.sendToMainQueue(msg, "RetryLoop: Messages queue full, dropping message")
	case <-q.ctx.Done():
		q.sendToMainQueue(msg, "RetryLoop: Dropping message due to shutdown during delay")
	}
}

// handleShutdownRetry processa retry imediato durante shutdown
func (q *Queue[T]) handleShutdownRetry(msg Message[T]) {
	log.Printf("RetryLoop: Processing retry message %s immediately (shutdown mode)", msg.ID)

	select {
	case q.messages <- msg:
		// Mensagem enviada para processamento
	default:
		log.Printf("RetryLoop: Dropping message %s due to shutdown", msg.ID)
	}
}

// sendToMainQueue tenta enviar mensagem para fila principal
func (q *Queue[T]) sendToMainQueue(msg Message[T], failureMsg string) {
	defer func() {
		if r := recover(); r != nil {
			// Canal foi fechado, descarta mensagem silenciosamente
			log.Printf("RetryLoop: Dropping message %s due to closed channel", msg.ID)
		}
	}()

	select {
	case q.messages <- msg:
		// Mensagem enviada com sucesso
	default:
		// Canal está fechado, cheio ou em shutdown - descarta mensagem
		log.Printf(failureMsg+" %s", msg.ID)
	}
}

// handleShutdownRetries processa mensagens restantes durante shutdown
func (q *Queue[T]) handleShutdownRetries() {
	defer func() {
		if r := recover(); r != nil {
			// Canal foi fechado durante shutdown, isso é esperado
			log.Println("RetryLoop: Messages channel closed during shutdown cleanup")
		}
	}()

	for {
		// Verifica se está em shutdown antes de tentar ler
		if q.isShuttingDown() {
			log.Println("RetryLoop: Context cancelled, shutting down")
			return
		}

		// Protege acesso ao retryQueue durante shutdown
		q.retryMutex.RLock()
		if q.isShuttingDown() {
			q.retryMutex.RUnlock()
			log.Println("RetryLoop: Context cancelled, shutting down")
			return
		}

		select {
		case msg, ok := <-q.retryQueue:
			q.retryMutex.RUnlock()
			if !ok {
				return
			}
			// Durante shutdown, processa imediatamente usando select não-bloqueante
			select {
			case q.messages <- msg:
				log.Printf("RetryLoop: Sent message %s for immediate processing due to shutdown", msg.ID)
			default:
				// Canal está fechado ou cheio - descarta mensagem
				log.Printf("RetryLoop: Dropping message %s due to shutdown", msg.ID)
			}
		default:
			q.retryMutex.RUnlock()
			log.Println("RetryLoop: Context cancelled, shutting down")
			return
		}
	}
}

// processMessage processa uma mensagem individual usando strategy pattern
func (q *Queue[T]) processMessage(msg Message[T], workerID int) {
	msg.Attempts++
	msg.LastTry = time.Now()

	// Cria contexto de processamento
	pc := NewProcessingContext(q, msg, workerID)

	// Executa processamento
	result := q.executeProcessing(pc)

	// Trata resultado baseado no sucesso/falha
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

	// Processamento falhou
	pc.LogError(result.Error)

	// Verifica se deve retentar
	if pc.ShouldRetry(result.Error) {
		q.handleRetryAttempt(pc)
	} else {
		pc.LogDrop(fmt.Sprintf("Dropping message %s after %d attempts",
			pc.Message.ID, pc.Message.Attempts))
	}
}

// handleRetryAttempt tenta recolocar mensagem na fila de retry
func (q *Queue[T]) handleRetryAttempt(pc *ProcessingContext[T]) {
	pc.LogRetryAction()

	// Verifica se está em shutdown
	if q.isShuttingDown() {
		pc.LogDrop("Retry queue unavailable, dropping message")
		return
	}

	// Protege acesso ao retryQueue durante shutdown
	q.retryMutex.RLock()
	defer q.retryMutex.RUnlock()

	// Verifica novamente após adquirir o lock
	if q.isShuttingDown() {
		pc.LogDrop("Retry queue unavailable, dropping message")
		return
	}

	select {
	case q.retryQueue <- pc.Message:
		// Mensagem enviada para retry
	case <-q.ctx.Done():
		pc.LogDrop("Retry queue unavailable, dropping message")
	default:
		pc.LogDrop("Retry queue full, dropping message")
	}
}

// Stats retorna estatísticas da fila
func (q *Queue[T]) Stats() QueueStats {
	return QueueStats{
		QueueSize:           len(q.messages),
		RetryQueueSize:      len(q.retryQueue),
		CircuitBreakerState: q.circuitBreaker.State(),
		Workers:             q.config.Workers,
	}
}
