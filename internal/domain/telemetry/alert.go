package telemetry

import "time"

// AlertType firmware AlertTipi aynası.
type AlertType string

const (
	AlertKritik  AlertType = "KRITIK"
	AlertUyari   AlertType = "UYARI"
	AlertAksiyon AlertType = "AKSIYON"
	AlertNormal  AlertType = "NORMAL"
)

// Alert cihazdan gelen uyarı (up/alert).
type Alert struct {
	CabinID string    `json:"cabin_id"`
	Ts      time.Time `json:"ts"`
	Type    AlertType `json:"type"`
	Message string    `json:"msg"`
}
