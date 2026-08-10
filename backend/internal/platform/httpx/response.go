package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id"`
}

func OK(c *gin.Context, data interface{}) {
	traceID := GetTraceID(c)
	c.JSON(http.StatusOK, Envelope{
		Code:    0,
		Message: "ok",
		Data:    data,
		TraceID: traceID,
	})
}

func Fail(c *gin.Context, code, message string, status int) {
	traceID := GetTraceID(c)
	c.JSON(status, Envelope{
		Code:    1,
		Message: message,
		Data:    map[string]string{"error_code": code},
		TraceID: traceID,
	})
}
