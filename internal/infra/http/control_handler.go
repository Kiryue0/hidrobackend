package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kiryue0/hidrobackend/internal/app/usecases"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

// ControlHandler manuel komut + config güncelleme uçları.
type ControlHandler struct {
	control *usecases.ControlService
}

// NewControlHandler handler üretir.
func NewControlHandler(control *usecases.ControlService) *ControlHandler {
	return &ControlHandler{control: control}
}

type commandRequest struct {
	Actuator string `json:"actuator" binding:"required"`
	State    *bool  `json:"state"`
	Speed    *int   `json:"speed"`
}

// SendCommand POST /api/cabins/:id/command
func (h *ControlHandler) SendCommand(c *gin.Context) {
	var req commandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek gövdesi"})
		return
	}
	err := h.control.SendActuatorCommand(c.Request.Context(), userIDFrom(c), c.Param("id"), usecases.CommandInput{
		Actuator: req.Actuator,
		State:    req.State,
		Speed:    req.Speed,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "komut gönderildi"})
}

type testRequest struct {
	Enabled *bool    `json:"enabled" binding:"required"`
	T       *float64 `json:"t"`
	H       *float64 `json:"h"`
	Tds     *float64 `json:"tds"`
	Ph      *float64 `json:"ph"`
}

// SendTest POST /api/cabins/:id/test — sahte telemetri enjeksiyonu (test modu).
// enabled=true iken t/h zorunludur; enabled=false cihaza normal moda dön der.
func (h *ControlHandler) SendTest(c *gin.Context) {
	var req testRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek gövdesi (enabled zorunlu)"})
		return
	}
	ctx := c.Request.Context()
	if !*req.Enabled {
		if err := h.control.SetTestMode(ctx, userIDFrom(c), c.Param("id"), false); err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"status": "normal moda dönüldü"})
		return
	}
	if req.T == nil || req.H == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test verisi için t ve h zorunlu"})
		return
	}
	in := usecases.TestInput{T: *req.T, H: *req.H}
	if req.Tds != nil {
		in.Tds = *req.Tds
	}
	if req.Ph != nil {
		in.Ph = *req.Ph
	}
	if err := h.control.SendTestReading(ctx, userIDFrom(c), c.Param("id"), in); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "test verisi gönderildi"})
}

type configRequest struct {
	Thresholds *cabin.Thresholds     `json:"thresholds"`
	Decision   *cabin.DecisionConfig `json:"decision"`
}

// UpdateConfig PUT /api/cabins/:id/config
func (h *ControlHandler) UpdateConfig(c *gin.Context) {
	var req configRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek gövdesi"})
		return
	}
	cab, err := h.control.UpdateCabinConfig(c.Request.Context(), userIDFrom(c), c.Param("id"), usecases.ConfigInput{
		Thresholds: req.Thresholds,
		Decision:   req.Decision,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toDetail(cab))
}
