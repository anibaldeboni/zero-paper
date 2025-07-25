# Sistema de Fila Genérico Atmosbyte

Este projeto implementa um sistema robusto de fila de mensagens **genérico** para processamento de dados de qualquer tipo, com suporte a retry automático, circuit breaker e integração específica com a API OpenWeather.

## 🎯 Principais Características

### **Sistema Genérico**

- **Tipo genérico**: Suporte a qualquer tipo de dados usando Go Generics
- **Type safety**: Verificação de tipos em tempo de compilação
- **Flexibilidade total**: Não limitado a dados meteorológicos

### **Sistema de Fila**

- **Workers configuráveis**: Número ajustável de workers para processamento paralelo
- **Buffer configurável**: Tamanho do buffer interno da fila
- **Graceful shutdown**: Finalização elegante aguardando processamento das mensagens pendentes

### **Sistema de Retry**

- **Política de retry configurável**: Máximo de tentativas, delay base e máximo
- **Backoff exponencial**: Aumento progressivo do tempo entre tentativas
- **Retry inteligente**: Só retenta erros HTTP 5xx e outros erros específicos

### **Circuit Breaker**

- **Proteção contra falhas**: Evita chamadas excessivas a serviços com problemas
- **Estados automáticos**: Fechado → Aberto → Semi-aberto
- **Configuração flexível**: Threshold de falhas e timeout configuráveis

## 📁 Estrutura do Projeto

```
atmosbyte/
├── queue/
│   ├── queue.go                    # Sistema principal de fila (genérico)
│   ├── measurement.go              # Tipos específicos para medições meteorológicas
│   ├── queue_test.go              # Testes para funcionalidade específica
│   └── generic_examples_test.go   # Exemplos de uso com tipos customizados
├── openweather/
│   └── openweather.go             # Cliente e worker OpenWeather
├── adapter.go                     # Adapter para integração OpenWeather
└── main.go                       # Exemplo de uso
```

## 🚀 Como Usar

### 1. Fila Genérica com Tipos Customizados

```go
// Defina seu tipo de dados
type OrderData struct {
    ID       string  `json:"id"`
    Amount   float64 `json:"amount"`
    Customer string  `json:"customer"`
}

// Implemente um worker
type OrderWorker struct{}

func (w *OrderWorker) Process(ctx context.Context, msg queue.Message[OrderData]) error {
    // Processa o pedido
    fmt.Printf("Processing order %s for %s: $%.2f\n",
        msg.Data.ID, msg.Data.Customer, msg.Data.Amount)
    return nil
}

// Use a fila
worker := &OrderWorker{}
config := queue.DefaultQueueConfig()
q := queue.NewQueue[OrderData](worker, config)

// Envia dados
order := OrderData{ID: "ORD001", Amount: 99.50, Customer: "João"}
q.Enqueue(order)
```

### 2. Usando WorkerFunc (Mais Simples)

```go
// Worker usando função anônima
workerFunc := queue.WorkerFunc[string](func(ctx context.Context, msg queue.Message[string]) error {
    fmt.Printf("Processing message: %s\n", msg.Data)
    return nil
})

q := queue.NewQueue[string](workerFunc, config)
q.Enqueue("Hello, World!")
```

### 3. Caso Específico: Dados Meteorológicos

```go
// Para medições meteorológicas, use os tipos helper
worker := &MyMeasurementWorker{}
q := queue.NewMeasurementQueue(worker, config)

measurement := queue.Measurement{
    Temperature: 25.5,
    Humidity:    60.0,
    Pressure:    1013,
}
q.Enqueue(measurement)
```

### 4. Integração OpenWeather (Caso Real)

```go
// Cliente OpenWeather
client, err := openweather.NewOpenWeatherClient(apiKey)
if err != nil {
    log.Fatal(err)
}

// Worker OpenWeather
owWorker := openweather.NewOpenWeatherWorker(client, stationID)
worker := NewOpenWeatherWorkerAdapter(owWorker)

// Fila específica para meteorologia
q := queue.NewMeasurementQueue(worker, config)
```

## 💡 Exemplos de Tipos Customizados

### Sistema de Eventos

```go
type EventData struct {
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Payload   map[string]interface{} `json:"payload"`
}

type EventWorker struct{}

func (w *EventWorker) Process(ctx context.Context, msg queue.Message[EventData]) error {
    switch msg.Data.Type {
    case "user.login":
        return w.handleLogin(msg.Data.Payload)
    case "order.created":
        return w.handleOrder(msg.Data.Payload)
    default:
        return w.handleGeneric(msg.Data)
    }
}

q := queue.NewQueue[EventData](worker, config)
```

### Sistema de Logs

```go
type LogEntry struct {
    Level     string    `json:"level"`
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
    Source    string    `json:"source"`
}

workerFunc := queue.WorkerFunc[LogEntry](func(ctx context.Context, msg queue.Message[LogEntry]) error {
    // Envia para Elasticsearch, arquivo, etc.
    return logStorage.Store(msg.Data)
})

q := queue.NewQueue[LogEntry](workerFunc, config)
```

## ⚙️ Type Aliases para Facilitar o Uso

O pacote fornece type aliases para casos comuns:

```go
// Para medições meteorológicas
type MeasurementQueue = Queue[Measurement]
type MeasurementMessage = Message[Measurement]
type MeasurementWorker = Worker[Measurement]

// Função helper
func NewMeasurementQueue(worker MeasurementWorker, config QueueConfig) *MeasurementQueue

// Você pode criar seus próprios aliases
type OrderQueue = queue.Queue[OrderData]
type EventQueue = queue.Queue[EventData]
type LogQueue = queue.Queue[LogEntry]
```

## ✅ Vantagens do Sistema Genérico

### Type Safety

```go
// Compile-time type checking
q := queue.NewQueue[OrderData](worker, config)
q.Enqueue(OrderData{...})          // ✅ OK
q.Enqueue("string")                 // ❌ Compile error
q.Enqueue(Measurement{...})         // ❌ Compile error
```

### Performance

- Sem boxing/unboxing de interfaces vazias
- Sem type assertions em runtime
- Memory layout otimizado

### Developer Experience

- IDE fornece sugestões precisas
- Refactoring seguro
- Documentação contextual

### Reutilização

```go
// Mesma infraestrutura para qualquer tipo
emailQueue := queue.NewQueue[EmailData](emailWorker, config)
orderQueue := queue.NewQueue[OrderData](orderWorker, config)
logQueue := queue.NewQueue[LogEntry](logWorker, config)
```

## 🔧 Execução e Testes

```bash
# Testes básicos
go test ./queue

# Testes com verbose
go test -v ./queue

# Executar exemplo específico
go test -v ./queue -run TestGenericQueue_CustomType

# Executar main (exemplo OpenWeather)
export OPENWEATHER_API_KEY="sua_chave"
export STATION_ID="sua_estacao"
go run .
```

## 🔄 Migração de Código Existente

### Antes (Não Genérico)

```go
type Worker interface {
    Process(ctx context.Context, data interface{}) error
}

func (w *MyWorker) Process(ctx context.Context, data interface{}) error {
    // Type assertion necessária
    myData, ok := data.(MyDataType)
    if !ok {
        return errors.New("invalid data type")
    }
    // processar myData...
}
```

### Depois (Genérico)

```go
type MyWorker struct{}

func (w *MyWorker) Process(ctx context.Context, msg queue.Message[MyDataType]) error {
    // Tipo já garantido, sem type assertion
    myData := msg.Data
    // processar myData...
}
```

## 📊 Monitoramento

```go
stats := q.Stats()
log.Printf("Queue: %d, Retry: %d, CircuitBreaker: %v, Workers: %d",
    stats.QueueSize,
    stats.RetryQueueSize,
    stats.CircuitBreakerState,
    stats.Workers,
)
```

## 🏗️ Boas Práticas Implementadas

### Go Standards

- ✅ Go Generics para type safety
- ✅ Interfaces pequenas e focadas
- ✅ Context para cancelamento
- ✅ Error wrapping
- ✅ Graceful shutdown
- ✅ Concurrent safe com mutexes
- ✅ Channels para comunicação

### Reliability Patterns

- ✅ Circuit Breaker Pattern
- ✅ Retry with Exponential Backoff
- ✅ Worker Pool Pattern
- ✅ Graceful Degradation
- ✅ Error Classification

Este sistema genérico oferece máxima flexibilidade mantendo type safety e performance, permitindo que seja usado para qualquer tipo de processamento de dados em fila!
