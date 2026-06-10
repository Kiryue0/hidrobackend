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
