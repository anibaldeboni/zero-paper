package main

import (
	"github.com/anibaldeboni/zero-paper/atmosbyte/bme280"
	"github.com/anibaldeboni/zero-paper/atmosbyte/queue"
)

// MeasurementQueue é um tipo alias para Queue[Measurement] para facilitar o uso
type MeasurementQueue = queue.Queue[bme280.Measurement]

// MeasurementMessage é um tipo alias para Message[Measurement] para facilitar o uso
type MeasurementMessage = queue.Message[bme280.Measurement]

// MeasurementWorker é um tipo alias para Worker[Measurement] para facilitar o uso
type MeasurementWorker = queue.Worker[bme280.Measurement]

// MeasurementWorkerFunc é um tipo alias para WorkerFunc[Measurement] para facilitar o uso
type MeasurementWorkerFunc = queue.WorkerFunc[bme280.Measurement]

// NewMeasurementQueue cria uma nova fila para Measurement
func NewMeasurementQueue(worker MeasurementWorker, config queue.QueueConfig) *MeasurementQueue {
	return queue.NewQueue(worker, config)
}
