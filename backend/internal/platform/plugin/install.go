package plugin

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func InstallAll(engine *gin.Engine, logger *zap.Logger) {
	InstallAllWithContext(engine, PluginContext{Logger: logger})
}

func InstallAllWithContext(engine *gin.Engine, pctx PluginContext) {
	if engine == nil {
		return
	}
	for _, p := range Registered() {
		installOne(engine, pctx, p)
	}
}

func installOne(engine *gin.Engine, pctx PluginContext, p Plugin) {
	defer func() {
		if rec := recover(); rec != nil {
			if pctx.Logger != nil {
				pctx.Logger.Error("plugin register panicked", zap.Any("panic", rec))
			}
		}
	}()
	if p == nil {
		return
	}
	if cp, ok := p.(ContextPlugin); ok {
		cp.RegisterWithContext(engine, pctx)
		return
	}
	p.Register(engine)
}
