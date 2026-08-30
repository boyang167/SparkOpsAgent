package http

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterRoutesPanicsWhenMountedTwiceOnSameEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterRoutes(engine)

	require.Panics(t, func() {
		RegisterRoutes(engine)
	})
}
