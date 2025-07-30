package queue

import (
	"context"
	"strconv"
	"time"
)

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

// generateID gera um ID único para a mensagem
func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
