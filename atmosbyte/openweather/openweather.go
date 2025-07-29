package openweather

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultBaseURL  = "https://api.openweathermap.org/data/3.0"
	measurementPath = "/measurements"
	defaultTimeout  = 30 * time.Second
)

// Erros customizados
var (
	ErrInvalidAppID     = errors.New("invalid app ID")
	ErrInvalidStationID = errors.New("invalid station ID")
	ErrInvalidLimit     = errors.New("limit must be greater than 0")
	ErrInvalidTimeRange = errors.New("from time must be before to time")
)

// AggregationType representa os tipos de agregação disponíveis
type AggregationType string

// String implementa a interface Stringer
func (a AggregationType) String() string {
	return string(a)
}

// Validate verifica se o tipo de agregação é válido
func (a AggregationType) Validate() error {
	switch a {
	case Minute, Hour, Day:
		return nil
	default:
		return fmt.Errorf("invalid aggregation type: %s", a)
	}
}

// Tipos de agregação suportados
const (
	Minute AggregationType = "m" // Agregação por minuto
	Hour   AggregationType = "h" // Agregação por hora
	Day    AggregationType = "d" // Agregação por dia
)

// Measurement representa uma medição meteorológica individual
type Measurement struct {
	StationID   string   `json:"station_id"`
	Dt          int64    `json:"dt"`
	Temperature float64  `json:"temperature"`
	WindSpeed   float64  `json:"wind_speed"`
	WindGust    float64  `json:"wind_gust"`
	Pressure    int64    `json:"pressure"`
	Humidity    float64  `json:"humidity"`
	Rain1h      float64  `json:"rain_1h"`
	Clouds      []Clouds `json:"clouds"`
}

// Validate verifica se a medição é válida
func (m *Measurement) Validate() error {
	if m.StationID == "" {
		return ErrInvalidStationID
	}
	if m.Dt <= 0 {
		return errors.New("invalid timestamp")
	}
	return nil
}

// Clouds representa informações sobre nuvens
type Clouds struct {
	Condition string `json:"condition"`
}

// MeasurementResponse representa a resposta da API para consultas de medições
type MeasurementResponse struct {
	Type          string        `json:"type"`
	Date          int64         `json:"date"`
	StationID     string        `json:"station_id"`
	Temp          Temperature   `json:"temp"`
	Humidity      Humidity      `json:"humidity"`
	Wind          Wind          `json:"wind"`
	Pressure      Pressure      `json:"pressure"`
	Precipitation Precipitation `json:"precipitation"`
}

// Temperature representa dados de temperatura agregados
type Temperature struct {
	Max     *float64 `json:"max,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Average *float64 `json:"average,omitempty"`
	Weight  *int     `json:"weight,omitempty"`
}

// Humidity representa dados de umidade agregados
type Humidity struct {
	Average *float64 `json:"average,omitempty"`
	Weight  *int     `json:"weight,omitempty"`
}

// Wind representa dados de vento agregados
type Wind struct {
	Deg   *int     `json:"deg,omitempty"`
	Speed *float64 `json:"speed,omitempty"`
}

// Pressure representa dados de pressão agregados
type Pressure struct {
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
	Average *float64 `json:"average,omitempty"`
	Weight  *int     `json:"weight,omitempty"`
}

// Precipitation representa dados de precipitação agregados
type Precipitation struct {
	// TODO: Adicionar campos conforme necessário
}

// ClientOption é uma função para configurar o cliente
type ClientOption func(*OpenWeatherClient)

// WithHTTPClient permite configurar um cliente HTTP customizado
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *OpenWeatherClient) {
		c.client = client
	}
}

// WithBaseURL permite configurar uma URL base customizada
func WithBaseURL(baseURL string) ClientOption {
	return func(c *OpenWeatherClient) {
		c.baseURL = baseURL
	}
}

// WithTimeout permite configurar um timeout customizado
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *OpenWeatherClient) {
		c.client.Timeout = timeout
	}
}

// OpenWeatherClient é o cliente para interagir com a API OpenWeather
type OpenWeatherClient struct {
	appID   string
	baseURL string
	client  *http.Client
}

// NewOpenWeatherClient cria uma nova instância do cliente OpenWeather
func NewOpenWeatherClient(appID string, opts ...ClientOption) (*OpenWeatherClient, error) {
	if appID == "" {
		return nil, ErrInvalidAppID
	}

	client := &OpenWeatherClient{
		appID:   appID,
		baseURL: defaultBaseURL,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	// Aplica as opções de configuração
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// SendMeasurement envia uma medição para a API OpenWeather
func (c *OpenWeatherClient) SendMeasurement(ctx context.Context, measurement Measurement) error {
	if err := measurement.Validate(); err != nil {
		return fmt.Errorf("invalid measurement: %w", err)
	}

	body, err := json.Marshal([]Measurement{measurement})
	if err != nil {
		return fmt.Errorf("failed to marshal measurement: %w", err)
	}

	requestURL, err := c.buildURL(measurementPath, url.Values{
		"appid": []string{c.appID},
	})
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			// Log error but don't override the main error
		}
	}()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return NewHTTPError(res.StatusCode, string(body))
	}

	return nil
}

// buildURL constrói uma URL completa para a API
func (c *OpenWeatherClient) buildURL(path string, params url.Values) (string, error) {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	baseURL.Path = path
	baseURL.RawQuery = params.Encode()

	return baseURL.String(), nil
}

// GetMeasurementOptions define opções para consultar medições
type GetMeasurementOptions struct {
	StationID string
	Type      AggregationType
	Limit     int64
	From      time.Time
	To        time.Time
}

// Validate verifica se as opções são válidas
func (opts *GetMeasurementOptions) Validate() error {
	if opts.StationID == "" {
		return ErrInvalidStationID
	}
	if err := opts.Type.Validate(); err != nil {
		return err
	}
	if opts.Limit <= 0 {
		return ErrInvalidLimit
	}
	if !opts.From.Before(opts.To) {
		return ErrInvalidTimeRange
	}
	return nil
}

// GetMeasurement consulta medições da API OpenWeather
func (c *OpenWeatherClient) GetMeasurement(ctx context.Context, opts GetMeasurementOptions) (*MeasurementResponse, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	params := url.Values{
		"station_id": []string{opts.StationID},
		"type":       []string{opts.Type.String()},
		"limit":      []string{strconv.FormatInt(opts.Limit, 10)},
		"from":       []string{strconv.FormatInt(opts.From.Unix(), 10)},
		"to":         []string{strconv.FormatInt(opts.To.Unix(), 10)},
		"appid":      []string{c.appID},
	}

	requestURL, err := c.buildURL(measurementPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			// Log error but don't override the main error
		}
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, NewHTTPError(res.StatusCode, string(body))
	}

	var measurementResponse MeasurementResponse
	if err := json.NewDecoder(res.Body).Decode(&measurementResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &measurementResponse, nil
}

// HTTPError representa um erro HTTP para integração com o sistema de retry
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// IsRetryable implementa a interface RetryableError
func (e *HTTPError) IsRetryable() bool {
	// Erros HTTP 5xx são retentáveis
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// NewHTTPError cria um novo erro HTTP
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}
