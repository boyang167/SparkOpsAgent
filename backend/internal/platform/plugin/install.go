package plugin

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func InstallAll(engine *gin.Engine, logger *zap.Logger) {
	if engine == nil {
		return
	}
	for _, p := range Registered() {
		installOne(engine, logger, p)
	}
}

func installOne(engine *gin.Engine, logger *zap.Logger, p Plugin) {
	defer func() {
		if rec := recover(); rec != nil {
			if logger != nil {
				logger.Error("plugin register panicked", zap.Any("panic", rec))
			}
		}
	}()
	if p != nil {
		p.Register(engine)
	}
}
