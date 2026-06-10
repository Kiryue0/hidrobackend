// Package telemetry: Telemetry bounded context.
// Reading bir aggregate DEĞİL; append-only fact'tir. Domain invariant'ı yoktur;
// doğrudan time-series deposuna yazılır (Bölüm 3.4).
package telemetry

import "time"

// SensorHealth firmware 'ok' sağlık bayraklarının aynası.
type SensorHealth struct {
	SHT bool `json:"sht"`
	RTC bool `json:"rtc"`
	TDS bool `json:"tds"`
	PH  bool `json:"ph"`
}

// HourlyReading saatlik ortalamalara downsample edilmiş ölçüm (grafik için).
type HourlyReading struct {
	Bucket      time.Time `json:"bucket"`
	Temperature float64   `json:"t"`
	Humidity    float64   `json:"h"`
	TDS         float64   `json:"tds"`
	PH          float64   `json:"ph"`
	Samples     int64     `json:"samples"`
}

// Reading tek bir telemetri ölçümü (up/sensors).
// JSON etiketleri WebSocket yayın formatı içindir (UI tüketir).
type Reading struct {
	CabinID     string       `json:"cabin_id"`
	Ts          time.Time    `json:"ts"`
	Temperature float64      `json:"t"`
	Humidity    float64      `json:"h"`
	TDS         float64      `json:"tds"`
	PH          float64      `json:"ph"`
	Health      SensorHealth `json:"ok"`
}
