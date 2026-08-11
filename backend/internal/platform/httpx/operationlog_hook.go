package httpx

import "github.com/gin-gonic/gin"

type operationLogHook struct {
	handler gin.HandlerFunc
}

var defaultOperationLogHook operationLogHook

func OperationLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if defaultOperationLogHook.handler != nil {
			defaultOperationLogHook.handler(c)
			return
		}
		c.Next()
	}
}

func SetOperationLogMiddleware(handler gin.HandlerFunc) {
	defaultOperationLogHook.handler = handler
}
