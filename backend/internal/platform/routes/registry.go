package routes

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type Route struct {
	Method string
	Path   string
}

type Registry struct {
	routes []Route
}

func Collect(engine *gin.Engine) *Registry {
	ginRoutes := engine.Routes()
	out := make([]Route, 0, len(ginRoutes))
	for _, item := range ginRoutes {
		path := item.Path
		if path == "/health" || strings.HasPrefix(path, "/metrics") || strings.HasPrefix(path, "/swagger") {
			continue
		}
		out = append(out, Route{Method: item.Method, Path: path})
	}
	return &Registry{routes: out}
}

func (r *Registry) All() []Route {
	if r == nil {
		return nil
	}
	return append([]Route(nil), r.routes...)
}
