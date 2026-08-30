package plugin

import "github.com/gin-gonic/gin"

// Plugin matches gin-vue-admin utils/plugin/v2.
type Plugin interface {
	Register(group *gin.Engine)
}
