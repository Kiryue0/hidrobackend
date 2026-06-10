package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kiryue0/hidrobackend/internal/app/usecases"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

// CabinHandler kabin CRUD + claim uçlarını sağlar.
type CabinHandler struct {
	cabins *usecases.CabinService
}

// NewCabinHandler handler üretir.
func NewCabinHandler(cabins *usecases.CabinService) *CabinHandler {
	return &CabinHandler{cabins: cabins}
}

// --- DTO'lar ---

type cabinSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Online   bool   `json:"online"`
	LastSeen *int64 `json:"last_seen,omitempty"` // unix saniye
}

type actuatorDTO struct {
	Type   string `json:"type"`
	On     bool   `json:"on"`
	Speed  int    `json:"speed"`
	Source string `json:"source"`
}

type cabinDetail struct {
	cabinSummary
	Thresholds cabin.Thresholds       `json:"thresholds"`
	Decision   cabin.DecisionConfig   `json:"decision"`
	Actuators  map[string]actuatorDTO `json:"actuators"`
}

func toSummary(c *cabin.Cabin) cabinSummary {
	s := cabinSummary{ID: c.ID().String(), Name: c.Name(), Online: c.Online()}
	if ls := c.LastSeen(); ls != nil {
		u := ls.Unix()
		s.LastSeen = &u
	}
	return s
}

func toDetail(c *cabin.Cabin) cabinDetail {
	acts := make(map[string]actuatorDTO, len(c.Actuators()))
	for t, st := range c.Actuators() {
		acts[string(t)] = actuatorDTO{Type: string(st.Type), On: st.On, Speed: st.Speed, Source: string(st.Source)}
	}
	return cabinDetail{
		cabinSummary: toSummary(c),
		Thresholds:   c.Thresholds(),
		Decision:     c.Decision(),
		Actuators:    acts,
	}
}

// --- Handler'lar ---

type createCabinRequest struct {
	ID   string `json:"id" binding:"required"`
	Name string `json:"name"`
}

// Create POST /api/cabins
func (h *CabinHandler) Create(c *gin.Context) {
	var req createCabinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek gövdesi"})
		return
	}
	cab, err := h.cabins.Create(c.Request.Context(), userIDFrom(c), req.ID, req.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toDetail(cab))
}

type claimRequest struct {
	ID string `json:"id" binding:"required"`
}

// Claim POST /api/cabins/claim
func (h *CabinHandler) Claim(c *gin.Context) {
	var req claimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek gövdesi"})
		return
	}
	cab, err := h.cabins.Claim(c.Request.Context(), userIDFrom(c), req.ID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toDetail(cab))
}

// List GET /api/cabins
func (h *CabinHandler) List(c *gin.Context) {
	cabs, err := h.cabins.List(c.Request.Context(), userIDFrom(c))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]cabinSummary, 0, len(cabs))
	for _, cab := range cabs {
		out = append(out, toSummary(cab))
	}
	c.JSON(http.StatusOK, gin.H{"cabins": out})
}

// Get GET /api/cabins/:id
func (h *CabinHandler) Get(c *gin.Context) {
	cab, err := h.cabins.Get(c.Request.Context(), userIDFrom(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toDetail(cab))
}
