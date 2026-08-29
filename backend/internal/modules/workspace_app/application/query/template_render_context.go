package query

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TemplateRenderContext struct {
	OrgName            string
	OrgSlug            string
	WorkspaceName      string
	WorkspaceSlug      string
	GitSecretName      string
	GitCredentialID    string
	RegistrySecretName string
	RegistryURL        string
	RegistryHost       string
	ImagePullSecret    string
	ContainerImage     string
}

func (h *GenerateProjectTemplatesHandler) loadRenderContext(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	project *model.Project,
) TemplateRenderContext {
	out := TemplateRenderContext{}
	if h.db != nil {
		var row struct {
			WorkspaceName string
			WorkspaceSlug string
			OrgName       string
			OrgSlug       string
		}
		err := h.db.WithContext(ctx).Raw(`
			SELECT w.name AS workspace_name, w.slug AS workspace_slug,
			       o.name AS org_name, o.slug AS org_slug
			FROM tenant.workspaces w
			JOIN tenant.organizations o ON o.id = w.org_id AND o.deleted_at IS NULL
			WHERE w.id = ? AND w.deleted_at IS NULL
		`, wsID.String()).Scan(&row).Error
		if err == nil {
			out.WorkspaceName = strings.TrimSpace(row.WorkspaceName)
			out.WorkspaceSlug = strings.TrimSpace(row.WorkspaceSlug)
			out.OrgName = strings.TrimSpace(row.OrgName)
			out.OrgSlug = strings.TrimSpace(row.OrgSlug)
		}
	}

	if out.WorkspaceSlug == "" {
		out.WorkspaceSlug = sanitizeTemplateSegment(project.Namespace)
	}
	if out.WorkspaceName == "" {
		out.WorkspaceName = out.WorkspaceSlug
	}
	if out.OrgSlug == "" {
		out.OrgSlug = "org"
	}
	if out.OrgName == "" {
		out.OrgName = out.OrgSlug
	}

	if h.secretRepo != nil {
		gitSecretID := strings.TrimSpace(project.SecretID)
		if gitSecretID != "" {
			if sid, err := kernel.ParseSecretID(gitSecretID); err == nil {
				if sec, err := h.secretRepo.FindByID(ctx, wsID, sid); err == nil && sec != nil {
					out.GitSecretName = sec.Name.String()
				}
			}
		}
		secretID := strings.TrimSpace(project.Resources.RegistrySecretID)
		if secretID != "" {
			if sid, err := kernel.ParseSecretID(secretID); err == nil {
				if sec, err := h.secretRepo.FindByID(ctx, wsID, sid); err == nil && sec != nil {
					out.RegistrySecretName = sec.Name.String()
					out.RegistryURL = strings.TrimSpace(sec.ServiceURL)
					out.RegistryHost = registryHostFromURL(out.RegistryURL)
				}
			}
		}
	}
	if out.RegistrySecretName == "" {
		out.RegistrySecretName = "registry"
	}
	if out.GitSecretName == "" {
		out.GitSecretName = "gitlab"
	}
	if out.RegistryHost == "" {
		out.RegistryHost = "harbor.local"
	}

	wsSegment := out.WorkspaceSlug
	if wsSegment == "" {
		wsSegment = sanitizeTemplateSegment(project.Namespace)
	}
	if wsSegment == "" {
		wsSegment = sanitizeTemplateSegment(project.Name.String())
	}
	out.GitCredentialID = fmt.Sprintf("git-%s-%s", sanitizeTemplateSegment(wsSegment), sanitizeTemplateSegment(out.GitSecretName))

	projectName := project.Name.String()
	gitRef := strings.TrimSpace(project.GitRefValue)
	if gitRef == "" {
		gitRef = "latest"
	}

	out.ContainerImage = fmt.Sprintf("%s/%s/%s:%s", out.RegistryHost, wsSegment, projectName, gitRef)
	out.ImagePullSecret = buildImagePullSecretName(out.OrgSlug, wsSegment, out.RegistrySecretName, projectName)
	return out
}

func buildImagePullSecretName(orgSlug, workspaceSlug, registrySecretName, projectName string) string {
	random4, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		random4 = big.NewInt(0)
	}
	parts := []string{
		sanitizeTemplateSegment(orgSlug),
		sanitizeTemplateSegment(workspaceSlug),
		sanitizeTemplateSegment(registrySecretName),
		sanitizeTemplateSegment(projectName),
		fmt.Sprintf("%04d", random4.Int64()),
	}
	name := strings.Join(parts, "-")
	if len(name) > 253 {
		name = name[:253]
	}
	return strings.Trim(name, "-")
}

func sanitizeTemplateSegment(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "na"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		case r == '_', r == '.', r == ' ':
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "na"
	}
	return out
}

func registryHostFromURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return strings.Trim(trimmed, "/")
	}
	return u.Host
}

func applyRenderContextVars(vars map[string]string, renderCtx TemplateRenderContext, projectName, gitRefValue string) {
	vars["org_name"] = renderCtx.OrgName
	vars["org_slug"] = renderCtx.OrgSlug
	vars["workspace_name"] = renderCtx.WorkspaceName
	vars["workspace_slug"] = renderCtx.WorkspaceSlug
	vars["registry_secret_name"] = renderCtx.RegistrySecretName
	vars["registry_url"] = renderCtx.RegistryURL
	vars["registry_host"] = renderCtx.RegistryHost
	vars["image_pull_secret_name"] = renderCtx.ImagePullSecret
	vars["imagesecret"] = renderCtx.ImagePullSecret
	vars["image_secret_name"] = renderCtx.ImagePullSecret
	vars["image"] = renderCtx.ContainerImage
	vars["image_name"] = renderCtx.ContainerImage
	vars["container_image"] = renderCtx.ContainerImage
	vars["hub_addr"] = renderCtx.RegistryHost
	vars["hub_namespace"] = renderCtx.WorkspaceSlug
	vars["HubAddr"] = renderCtx.RegistryHost
	vars["HubNameSpace"] = renderCtx.WorkspaceSlug
	vars["ProjectName"] = projectName
	vars["CodeVersion"] = gitRefValue
	vars["git_version"] = gitRefValue
	vars["git_credential_id"] = renderCtx.GitCredentialID
	vars["CodeCrendentialId"] = renderCtx.GitCredentialID
}

func applyRenderContextData(data map[string]interface{}, renderCtx TemplateRenderContext, projectName, gitRefValue string) {
	data["org_name"] = renderCtx.OrgName
	data["org_slug"] = renderCtx.OrgSlug
	data["workspace_name"] = renderCtx.WorkspaceName
	data["workspace_slug"] = renderCtx.WorkspaceSlug
	data["registry_secret_name"] = renderCtx.RegistrySecretName
	data["registry_url"] = renderCtx.RegistryURL
	data["registry_host"] = renderCtx.RegistryHost
	data["registry_credential_id"] = renderCtx.RegistrySecretName
	data["image_pull_secret_name"] = renderCtx.ImagePullSecret
	data["imagePullSecretName"] = renderCtx.ImagePullSecret
	data["ImagePullSecretName"] = renderCtx.ImagePullSecret
	data["imagesecret"] = renderCtx.ImagePullSecret
	data["image_secret_name"] = renderCtx.ImagePullSecret
	data["image"] = renderCtx.ContainerImage
	data["Image"] = renderCtx.ContainerImage
	data["image_name"] = renderCtx.ContainerImage
	data["container_image"] = renderCtx.ContainerImage
	data["hub_addr"] = renderCtx.RegistryHost
	data["hub_namespace"] = renderCtx.WorkspaceSlug
	data["HubAddr"] = renderCtx.RegistryHost
	data["HubNameSpace"] = renderCtx.WorkspaceSlug
	data["ProjectName"] = projectName
	data["CodeVersion"] = gitRefValue
	data["git_version"] = gitRefValue
	data["git_credential_id"] = renderCtx.GitCredentialID
	data["CodeCrendentialId"] = renderCtx.GitCredentialID
}
