package httpx

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const traceIDKey = "trace_id"

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := c.GetHeader("X-Trace-Id")
		if tid == "" {
			tid = uuid.NewString()
		}
		c.Header("X-Trace-Id", tid)
		c.Set(traceIDKey, tid)
		c.Next()
	}
}

func GetTraceID(c *gin.Context) string {
	if v, ok := c.Get(traceIDKey); ok {
		if tid, ok := v.(string); ok {
			return tid
		}
	}
	return ""
}
