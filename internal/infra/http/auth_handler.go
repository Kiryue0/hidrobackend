package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/usecases"
)

// AuthHandler kayıt/giriş uçlarını sağlar.
type AuthHandler struct {
	auth *usecases.AuthService
}

// NewAuthHandler handler üretir.
func NewAuthHandler(auth *usecases.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type userResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Register POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek gövdesi"})
		return
	}
	u, err := h.auth.Register(c.Request.Context(), usecases.RegisterInput{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, userResponse{ID: u.ID(), Username: u.Username(), Email: u.Email()})
}

type loginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // username veya email
	Password   string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz istek gövdesi"})
		return
	}
	token, err := h.auth.Login(c.Request.Context(), usecases.LoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token})
}

// DeleteAccount DELETE /api/account
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID := userIDFrom(c)
	if err := h.auth.DeleteAccount(c.Request.Context(), userID); err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			respondError(c, apperr.ErrNotFound)
			return
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
