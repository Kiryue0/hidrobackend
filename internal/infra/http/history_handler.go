package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kiryue0/hidrobackend/internal/app/usecases"
)

// HistoryHandler sensör geçmişi (read model) uçları.
type HistoryHandler struct {
	history *usecases.HistoryService
}

// NewHistoryHandler handler üretir.
func NewHistoryHandler(history *usecases.HistoryService) *HistoryHandler {
	return &HistoryHandler{history: history}
}

// GetReadings GET /api/cabins/:id/readings?from=<unix>&to=<unix>&limit=<n>&bucket=raw|hour
func (h *HistoryHandler) GetReadings(c *gin.Context) {
	q := usecases.HistoryQuery{
		From:   parseUnix(c.Query("from")),
		To:     parseUnix(c.Query("to")),
		Hourly: c.Query("bucket") == "hour",
	}
	if l, err := strconv.Atoi(c.Query("limit")); err == nil {
		q.Limit = int32(l)
	}

	res, err := h.history.GetSensorHistory(c.Request.Context(), userIDFrom(c), c.Param("id"), q)
	if err != nil {
		respondError(c, err)
		return
	}
	if q.Hourly {
		c.JSON(http.StatusOK, gin.H{"bucket": "hour", "readings": res.Hourly})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bucket": "raw", "readings": res.Raw})
}

// parseUnix unix saniye query param'ını time.Time'a çevirir (boş/geçersiz -> zero).
func parseUnix(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil || sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}
