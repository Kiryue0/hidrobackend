// Package http: Gin tabanlı primary (driving) adapter.
package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
)

// respondError application hatasını uygun HTTP durum koduna eşler.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperr.ErrValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, apperr.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "kimlik bilgileri hatalı"})
	case errors.Is(err, apperr.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "yetkisiz"})
	case errors.Is(err, apperr.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "bulunamadı"})
	case errors.Is(err, apperr.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "çakışma: kayıt zaten var"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sunucu hatası"})
	}
}
