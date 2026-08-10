package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/redis"
)

type ReadinessDeps struct {
	DB    *gorm.DB
	Redis *redis.Client
}

func RegisterReadiness(r *gin.Engine, deps ReadinessDeps) {
	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		checks := map[string]string{}
		ok := true

		if deps.DB == nil {
			checks["database"] = "missing"
			ok = false
		} else if sqlDB, err := deps.DB.DB(); err != nil {
			checks["database"] = "error"
			ok = false
		} else if err := sqlDB.PingContext(ctx); err != nil {
			checks["database"] = "down"
			ok = false
		} else {
			checks["database"] = "up"
		}

		if deps.Redis != nil {
			if err := deps.Redis.Ping(ctx); err != nil {
				checks["redis"] = "down"
			} else {
				checks["redis"] = "up"
			}
		} else {
			checks["redis"] = "skipped"
		}

		if !ok {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    "NOT_READY",
				"message": "dependencies unavailable",
				"data":    checks,
			})
			return
		}
		OK(c, gin.H{"status": "ready", "checks": checks})
	})
}
