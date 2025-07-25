package queue

// Measurement representa uma medição meteorológica - tipo específico para demonstração
type Measurement struct {
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	Pressure    int64   `json:"pressure"`
}

// MeasurementQueue é um tipo alias para Queue[Measurement] para facilitar o uso
type MeasurementQueue = Queue[Measurement]

// MeasurementMessage é um tipo alias para Message[Measurement] para facilitar o uso
type MeasurementMessage = Message[Measurement]

// MeasurementWorker é um tipo alias para Worker[Measurement] para facilitar o uso
type MeasurementWorker = Worker[Measurement]

// MeasurementWorkerFunc é um tipo alias para WorkerFunc[Measurement] para facilitar o uso
type MeasurementWorkerFunc = WorkerFunc[Measurement]

// NewMeasurementQueue cria uma nova fila para Measurement
func NewMeasurementQueue(worker MeasurementWorker, config QueueConfig) *MeasurementQueue {
	return NewQueue[Measurement](worker, config)
}
