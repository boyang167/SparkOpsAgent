package knowledge

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	knowledgehttp "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "knowledge" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("knowledge module requires database")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("knowledge migrate: %w", err)
	}

	repo := persistence.NewKnowledgeRepository(c.DB)
	wsChecker := acl.NewTenantACL(c.DB)

	handler := knowledgehttp.NewHandler(
		command.NewCreateKnowledgeBaseHandler(repo, wsChecker),
		command.NewUpdateKnowledgeBaseHandler(repo, wsChecker),
		command.NewArchiveKnowledgeBaseHandler(repo, wsChecker),
		command.NewCreateDocumentHandler(repo, wsChecker),
		command.NewUpdateDocumentHandler(repo, wsChecker),
		command.NewArchiveDocumentHandler(repo, wsChecker),
		query.NewListKnowledgeBasesHandler(repo, wsChecker),
		query.NewGetKnowledgeBaseHandler(repo, wsChecker),
		query.NewListDocumentsHandler(repo, wsChecker),
		query.NewGetDocumentHandler(repo, wsChecker),
		query.NewSearchDocumentsHandler(repo, wsChecker),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/workspaces/:wsId/knowledge-bases",
		auth, ws, wsMember, handler.ListKnowledgeBases)
	c.Engine.POST("/api/v1/workspaces/:wsId/knowledge-bases",
		auth, ws, wsMember, handler.CreateKnowledgeBase)
	c.Engine.GET("/api/v1/knowledge-bases/:kbId",
		auth, ws, wsMember, handler.GetKnowledgeBase)
	c.Engine.PATCH("/api/v1/knowledge-bases/:kbId",
		auth, ws, wsMember, handler.UpdateKnowledgeBase)
	c.Engine.DELETE("/api/v1/knowledge-bases/:kbId",
		auth, ws, wsMember, handler.ArchiveKnowledgeBase)
	c.Engine.GET("/api/v1/knowledge-bases/:kbId/documents",
		auth, ws, wsMember, handler.ListDocuments)
	c.Engine.POST("/api/v1/knowledge-bases/:kbId/documents",
		auth, ws, wsMember, handler.CreateDocument)
	c.Engine.POST("/api/v1/knowledge-bases/:kbId/search",
		auth, ws, wsMember, handler.SearchDocuments)
	c.Engine.GET("/api/v1/documents/:docId",
		auth, ws, wsMember, handler.GetDocument)
	c.Engine.PATCH("/api/v1/documents/:docId",
		auth, ws, wsMember, handler.UpdateDocument)
	c.Engine.DELETE("/api/v1/documents/:docId",
		auth, ws, wsMember, handler.ArchiveDocument)

	return nil
}
