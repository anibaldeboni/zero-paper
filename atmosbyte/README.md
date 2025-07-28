# Atmosbyte - Sistema de Processamento de Dados Meteorológicos

Este projeto implementa um sistema completo de coleta, processamento e envio de dados meteorológicos usando Go, integrando sensor BME280, sistema de fila genérico com retry/circuit breaker e API OpenWeather.

## 🎯 Principais Características

### **Arquitetura Modular**
- **Package BME280**: Leitura de sensor hardware com encapsulamento limpo
- **Package Queue**: Sistema genérico de fila com retry e circuit breaker
- **Package OpenWeather**: Cliente HTTP para API OpenWeather
- **Coordenação Central**: `main.go` orquestra todos os componentes

### **Flexibilidade de Fonte de Dados**
- **Sensor Real**: BME280 via I2C para temperatura, umidade e pressão
- **Sensor Simulado**: Geração de dados realistas quando hardware não disponível
- **Fallback Automático**: Detecta hardware e alterna para simulação se necessário

### **Sistema de Fila Robusto**
- **Retry inteligente**: Interface `RetryableError` para lógica personalizada
- **Circuit breaker**: Proteção contra falhas da API externa
- **Processamento paralelo**: Workers configuráveis
- **Graceful shutdown**: Finalização elegante com processamento de mensagens pendentes

## 📁 Estrutura do Projeto

```
atmosbyte/
├── bme280/                 # Package do sensor BME280
│   ├── bme280.go          # Implementação principal
│   ├── bme280_test.go     # Testes unitários
│   ├── example/           # Exemplo de uso
│   └── README.md          # Documentação específica
├── queue/                  # Sistema de fila genérico
│   ├── queue.go           # Implementação principal
│   ├── measurement.go     # Tipos para medições
│   └── queue_test.go      # Testes completos
├── openweather/           # Cliente OpenWeather API
│   └── openweather.go     # Cliente HTTP e workers
├── adapter.go             # Adapter entre OpenWeather e queue
├── sensor_worker.go       # Workers para coleta de dados
├── measurement.go         # Type aliases para facilitar uso
├── main.go               # Coordenação principal
└── .env.example          # Exemplo de configuração
```

## 🚀 Quick Start

### 1. Instalação

```bash
# Clone o repositório
git clone https://github.com/anibaldeboni/zero-paper.git
cd zero-paper/atmosbyte

# Instale dependências do BME280 (se usar hardware real)
go get periph.io/x/conn/v3/...
go get periph.io/x/devices/v3/...
go get periph.io/x/host/v3

# Compile o projeto
go build .
```

### 2. Configuração

```bash
# Copie o arquivo de exemplo
cp .env.example .env

# Configure suas credenciais
export OPENWEATHER_API_KEY="sua_chave_api"
export STATION_ID="sua_estacao_meteorologica"

# Para usar sensor simulado (padrão para desenvolvimento)
export USE_SIMULATED_SENSOR="true"

# Para usar BME280 real (Raspberry Pi com hardware)
export USE_SIMULATED_SENSOR="false"
```

### 3. Execução

```bash
# Desenvolvimento (sensor simulado)
USE_SIMULATED_SENSOR=true go run .

# Produção (BME280 real)
USE_SIMULATED_SENSOR=false go run .

# Com logs detalhados
go run . -v
```

## 💻 Exemplos de Uso

### Sensor Simulado (Desenvolvimento)
```bash
$ USE_SIMULATED_SENSOR=true go run .
2025/07/28 11:45:00 Starting Atmosbyte - Weather Data Processing System
2025/07/28 11:45:00 Using simulated sensor data
2025/07/28 11:45:00 Starting simulated sensor worker (generating data every 10s)
2025/07/28 11:45:10 Simulated reading enqueued: temp=27.5°C, humidity=62.3%, pressure=101250 Pa
2025/07/28 11:45:10 Worker 0: Successfully processed message 1753713910123456000
```

### BME280 Hardware (Produção)
```bash
$ USE_SIMULATED_SENSOR=false go run .
2025/07/28 11:45:00 Starting Atmosbyte - Weather Data Processing System
2025/07/28 11:45:00 Attempting to use BME280 hardware sensor
2025/07/28 11:45:00 BME280 sensor initialized successfully
2025/07/28 11:45:00 Starting BME280 sensor worker (reading every 10s)
2025/07/28 11:45:10 BME280 reading enqueued: temp=23.4°C, humidity=58.2%, pressure=101325 Pa
2025/07/28 11:45:10 Worker 0: Successfully processed message 1753713910123456000
```

### Fallback Automático
```bash
$ USE_SIMULATED_SENSOR=false go run .
2025/07/28 11:45:00 Starting Atmosbyte - Weather Data Processing System
2025/07/28 11:45:00 Attempting to use BME280 hardware sensor
2025/07/28 11:45:00 Failed to initialize BME280 sensor: failed to open I2C bus
2025/07/28 11:45:00 Falling back to simulated sensor data
2025/07/28 11:45:00 Starting simulated sensor worker (generating data every 10s)
```

## 🔧 Configuração Avançada

### Variáveis de Ambiente

| Variável | Descrição | Padrão | Obrigatória |
|----------|-----------|---------|-------------|
| `OPENWEATHER_API_KEY` | Chave da API OpenWeather | - | ✅ |
| `STATION_ID` | ID da estação meteorológica | - | ✅ |
| `USE_SIMULATED_SENSOR` | Usar sensor simulado | `false` | ❌ |

### Configuração do BME280

```go
// Configuração customizada do sensor
config := &bme280.Config{
    Address: 0x77,           // Endereço I2C alternativo
    BusName: "/dev/i2c-1",   // Bus específico
    Options: customOpts,     // Opções de precisão
}
```

### Configuração da Fila

```go
config := queue.DefaultQueueConfig()
config.Workers = 4                        // 4 workers paralelos
config.BufferSize = 100                   // Buffer de 100 mensagens
config.RetryPolicy.MaxRetries = 5         // Máximo 5 tentativas
config.RetryPolicy.BaseDelay = 10 * time.Second
```

## 🧪 Testes

```bash
# Testes completos
go test ./...

# Testes com verbose
go test -v ./...

# Testes específicos
go test -v ./queue
go test -v ./bme280

# Benchmarks
go test -bench=. ./...

# Coverage
go test -cover ./...
```

## 📊 Monitoramento

O sistema fornece logs estruturados e estatísticas em tempo real:

```
📊 Queue stats - Size: 2, Retry: 0, CircuitBreaker: Closed, Workers: 2
```

### Métricas Disponíveis
- **QueueSize**: Mensagens na fila principal
- **RetryQueueSize**: Mensagens aguardando retry
- **CircuitBreakerState**: Estado do circuit breaker (Closed/Open/HalfOpen)
- **Workers**: Número de workers ativos

## 🛠️ Desenvolvimento

### Adicionando Novos Sensores

1. Implemente a interface `SensorWorker`:
```go
type CustomSensorWorker struct {
    // campos específicos
}

func (w *CustomSensorWorker) Start(ctx context.Context) error {
    // lógica de leitura contínua
}

func (w *CustomSensorWorker) Stop() {
    // cleanup
}
```

2. Registre no `main.go`:
```go
customWorker := NewCustomSensorWorker(params)
sensorManager := NewSensorManager(customWorker)
```

### Adicionando Novos Destinos

1. Implemente a interface `queue.Worker[bme280.Measurement]`:
```go
type CustomDestinationWorker struct{}

func (w *CustomDestinationWorker) Process(ctx context.Context, msg queue.Message[bme280.Measurement]) error {
    // processar measurement
    return nil
}
```

2. Crie adapter se necessário e registre na fila.

## 🔍 Troubleshooting

### Sensor BME280 não encontrado
```
Failed to initialize BME280 sensor: failed to open I2C bus
```
**Soluções:**
1. Verificar se I2C está habilitado: `sudo raspi-config`
2. Verificar conexões físicas
3. Usar `i2cdetect -y 1` para encontrar dispositivos
4. Sistema fará fallback automático para simulação

### Erro de API OpenWeather
```
Worker 0: Error processing message: HTTP 401: Invalid API key
```
**Soluções:**
1. Verificar `OPENWEATHER_API_KEY`
2. Confirmar que a chave tem acesso à API 3.0
3. Verificar `STATION_ID` correto

### Circuit Breaker Aberto
```
Queue stats - CircuitBreaker: Open
```
**Soluções:**
1. Aguardar timeout de recovery
2. Verificar conectividade com API
3. Verificar logs de erro detalhados

## 🏗️ Arquitetura Detalhada

### Fluxo de Dados

```
[BME280/Simulado] → [SensorWorker] → [Queue] → [OpenWeatherWorker] → [API]
                      ↓
                [bme280.Measurement]
                      ↓
            [Queue com Retry/CircuitBreaker]
                      ↓
              [OpenWeatherAdapter]
                      ↓
                [HTTP Request]
```

### Desacoplamento

1. **BME280 ↔ OpenWeather**: Não há dependência direta
2. **Queue ↔ Sensors**: Interface genérica
3. **Main ↔ Modules**: Coordenação sem acoplamento
4. **Adapters**: Conversão entre formatos de dados

### Princípios Aplicados

- ✅ **Single Responsibility**: Cada package tem responsabilidade única
- ✅ **Open/Closed**: Extensível sem modificar código existente
- ✅ **Dependency Inversion**: Depende de interfaces, não implementações
- ✅ **Interface Segregation**: Interfaces pequenas e focadas
- ✅ **DRY**: Código reutilizável e componível

## 📋 Checklist de Produção

### Hardware
- [ ] Raspberry Pi configurado com I2C habilitado
- [ ] BME280 conectado corretamente
- [ ] Alimentação estável
- [ ] Conectividade de rede

### Software
- [ ] Go 1.18+ instalado
- [ ] Dependências periph.io instaladas
- [ ] Variáveis de ambiente configuradas
- [ ] Permissões I2C corretas
- [ ] Logs configurados

### API
- [ ] Chave OpenWeather válida
- [ ] Station ID correto
- [ ] Limite de requisições verificado
- [ ] Conectividade testada

## 🔗 Referências

- [BME280 Package](./bme280/README.md) - Documentação específica do sensor
- [Queue System](./queue/) - Sistema de fila genérico
- [OpenWeather API](https://openweathermap.org/api) - Documentação da API
- [periph.io](https://periph.io/) - Biblioteca de hardware para Go

## 📈 Roadmap

- [ ] **Metrics**: Integração com Prometheus/Grafana
- [ ] **Configuração**: Arquivo YAML/TOML para configurações
- [ ] **Database**: Persistência local com SQLite
- [ ] **Web UI**: Dashboard web para monitoramento
- [ ] **Docker**: Containerização para deploy simplificado
- [ ] **Alertas**: Notificações para condições específicas

Este sistema fornece uma base sólida e extensível para processamento de dados meteorológicos em tempo real! 🌡️📊
