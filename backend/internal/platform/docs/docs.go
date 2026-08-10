package docs

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"

	openapispec "github.com/sparkops/sparkops-agent/backend/api/openapi"
)

const specBasePath = "/api/docs/openapi"

// Register mounts Swagger UI and embedded OpenAPI specs on the engine.
func Register(r *gin.Engine) {
	specFS, err := fs.Sub(openapispec.Files, ".")
	if err != nil {
		panic("docs: openapi embed: " + err.Error())
	}

	r.GET("/api/docs", serveUI)
	r.GET("/api/docs/", serveUI)
	r.StaticFS(specBasePath, http.FS(specFS))
}

func serveUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, swaggerUIHTML)
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>SparkOps API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "` + specBasePath + `/openapi.yaml",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout",
    });
  </script>
</body>
</html>`
