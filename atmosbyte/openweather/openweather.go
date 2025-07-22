package openweather

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	OpenWeatherUrl  = "https://api.openweathermap.org/data/3.0"
	MeasurementPath = "/measurements"
)

type AggregationType string

func (a AggregationType) String() string {
	return string(a)
}

const (
	Minute AggregationType = "m"
	Hour   AggregationType = "h"
	Day    AggregationType = "d"
)

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

type Clouds struct {
	Condition string `json:"condition"`
}

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

type Temperature struct {
	Max     *float64 `json:"max,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Average *float64 `json:"average,omitempty"`
	Weight  *int     `json:"weight,omitempty"`
}

type Humidity struct {
	Average *float64 `json:"average,omitempty"`
	Weight  *int     `json:"weight,omitempty"`
}

type Wind struct {
	Deg   *int     `json:"deg,omitempty"`
	Speed *float64 `json:"speed,omitempty"`
}

type Pressure struct {
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
	Average *float64 `json:"average,omitempty"`
	Weight  *int     `json:"weight,omitempty"`
}

type Precipitation struct {
	// Campos vazios no JSON, adicione conforme necessário
}

type OpenWeatherClient struct {
	appID  string
	client *http.Client
}

func NewOpenWeatherClient(appID string) *OpenWeatherClient {
	return &OpenWeatherClient{
		appID:  appID,
		client: &http.Client{},
	}
}

func (c *OpenWeatherClient) SendMeasurement(measurement Measurement) (*http.Response, error) {
	body, err := json.Marshal([]Measurement{measurement})
	if err != nil {
		return nil, err
	}

	urlStr := OpenWeatherUrl + MeasurementPath + "?appid=" + c.appID
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return res, nil
}

func (c *OpenWeatherClient) GetMeasurement(stationID string, aggType AggregationType, limit int64, from, to time.Time) (*MeasurementResponse, error) {
	qs := fmt.Sprintf("?station_id=%s&type=%s&limit=%d&from=%d&to=%d", stationID, aggType, limit, from.Unix(), to.Unix())
	urlStr := OpenWeatherUrl + MeasurementPath + qs + "&appid=" + c.appID
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get measurement: %s", res.Status)
	}

	var measurementResponse MeasurementResponse
	if err := json.NewDecoder(res.Body).Decode(&measurementResponse); err != nil {
		return nil, err
	}

	return &measurementResponse, nil
}
