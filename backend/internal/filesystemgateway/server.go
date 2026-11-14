package filesystemgateway

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Options struct {
	CapabilitySecret         string
	CapabilityAudience       string
	CapabilitySecretFallback string
	MaxFileBytes             int64
	Control                  ControlPort
	NonceStore               NonceStore
	AllowedOrigins           []string
}

type Server struct {
	auth         *CapabilityAuth
	control      ControlPort
	maxFileBytes int64
	origins      map[string]struct{}
}

func New(opts Options) (*Server, error) {
	secret := opts.CapabilitySecret
	if secret == "" {
		secret = opts.CapabilitySecretFallback
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = 20 * 1024 * 1024 * 1024
	}
	if opts.Control == nil {
		return nil, errors.New("control port is required")
	}
	auth, err := NewCapabilityAuth(Options{
		CapabilitySecret:   secret,
		CapabilityAudience: opts.CapabilityAudience,
		NonceStore:         opts.NonceStore,
	})
	if err != nil {
		return nil, err
	}
	return &Server{
		auth:         auth,
		control:      opts.Control,
		maxFileBytes: opts.MaxFileBytes,
		origins:      allowedOriginSet(opts.AllowedOrigins),
	}, nil
}

func (s *Server) RegisterRoutes(engine *gin.Engine) {
	engine.Use(func(c *gin.Context) {
		if origin := strings.TrimSpace(c.GetHeader("Origin")); origin != "" {
			if _, allowed := s.origins[origin]; !allowed {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	engine.GET("/health", func(c *gin.Context) {
		httpx.OK(c, gin.H{"status": "up"})
	})
	engine.GET("/ready", func(c *gin.Context) {
		if s.control == nil {
			httpx.Fail(c, "NOT_READY", "control port unavailable", http.StatusServiceUnavailable)
			return
		}
		httpx.OK(c, gin.H{"status": "ready"})
	})
	engine.PUT("/v1/transfers/:sessionId/content", s.handleUpload)
	engine.GET("/v1/files/:fileId/content", s.handleDownload)
}

func allowedOriginSet(origins []string) map[string]struct{} {
	result := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		if origin = strings.TrimSpace(origin); origin != "" {
			result[origin] = struct{}{}
		}
	}
	return result
}

func (s *Server) failCapability(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpx.Fail(c, "FORBIDDEN", "capability scope mismatch", http.StatusForbidden)
	case errors.Is(err, ErrCapabilityReplay), errors.Is(err, ErrCapabilityExpired), errors.Is(err, ErrInvalidCapability):
		httpx.Fail(c, "UNAUTHORIZED", err.Error(), http.StatusUnauthorized)
	default:
		httpx.Fail(c, "FORBIDDEN", "capability rejected", http.StatusForbidden)
	}
}

func bearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}
