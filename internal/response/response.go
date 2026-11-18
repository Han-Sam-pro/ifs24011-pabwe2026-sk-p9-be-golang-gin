// Package response menyediakan format JSON standar {status, message, data?}.
package response

import "github.com/gin-gonic/gin"

// Body adalah struktur respons JSON standar.
type Body struct {
	Status  string `json:"status" example:"success"`
	Message string `json:"message" example:"Berhasil"`
	Data    any    `json:"data,omitempty"`
}

// Success mengirim respons dengan status "success".
func Success(c *gin.Context, code int, message string, data any) {
	c.JSON(code, Body{Status: "success", Message: message, Data: data})
}

// Fail mengirim respons kesalahan klien (4xx) dengan status "fail".
func Fail(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "fail", Message: message})
}

// Error mengirim respons kesalahan server (5xx) dengan status "error".
func Error(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "error", Message: message})
}
