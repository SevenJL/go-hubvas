package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the unified API response envelope.
// Convention: { "code", "message", "data" }
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK sends a 200 success response.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// Created sends a 201 response.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// Error sends an error response with the given HTTP status and domain error code.
func Error(c *gin.Context, httpStatus int, code, message string) {
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:    httpStatus,
		Message: message,
	})
}

// BadRequest sends a 400 error.
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "bad_request", message)
}

// Unauthorized sends a 401 error.
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "unauthorized", message)
}

// Forbidden sends a 403 error.
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "forbidden", message)
}

// NotFound sends a 404 error.
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "not_found", message)
}

// Conflict sends a 409 error.
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, "conflict", message)
}

// InternalError sends a 500 error.
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "internal_error", message)
}
