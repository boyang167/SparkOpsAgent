package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func StreamSSE(c *gin.Context, ch <-chan string, initial string, done bool) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	writeEvent := func(payload map[string]string) {
		raw, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
		flusher.Flush()
	}

	if initial != "" {
		writeEvent(map[string]string{"delta": initial})
	}
	if done {
		writeEvent(map[string]string{"done": "true"})
		return
	}

	for {
		select {
		case delta, open := <-ch:
			if !open {
				writeEvent(map[string]string{"done": "true"})
				return
			}
			writeEvent(map[string]string{"delta": delta})
		case <-c.Request.Context().Done():
			return
		}
	}
}
