package queue

import (
	"sync"
	"time"
)

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
