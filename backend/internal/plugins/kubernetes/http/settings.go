package http

import (
	"context"
	"errors"
	"net/url"
	"strings"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
)

const maskedSecret = "***"

var errInvalidSettingsURL = errors.New("settings URL must be an absolute http or https URL without credentials")

type clusterSettingsRequest struct {
	PrometheusURL      string `json:"prometheus_url"`
	PrometheusUser     string `json:"prometheus_user"`
	PrometheusPassword string `json:"prometheus_password"`
	VelauxURL          string `json:"velaux_url"`
	VelauxUser         string `json:"velaux_user"`
	VelauxPassword     string `json:"velaux_password"`
	LLMBaseURL         string `json:"llm_base_url"`
	LLMAPIKey          string `json:"llm_api_key"`
	LLMModel           string `json:"llm_model"`
}

func clusterIDs(metas []sparkplugin.ClusterMeta) []string {
	ids := make([]string, 0, len(metas))
	for _, meta := range metas {
		ids = append(ids, meta.ID)
	}
	return ids
}

func applyRequest(settings *k8sschema.ClusterSettings, req clusterSettingsRequest) {
	settings.PrometheusURL = req.PrometheusURL
	settings.PrometheusUser = req.PrometheusUser
	settings.PrometheusPassword = updateSecret(settings.PrometheusPassword, req.PrometheusPassword)
	settings.VelauxURL = req.VelauxURL
	settings.VelauxUser = req.VelauxUser
	settings.VelauxPassword = updateSecret(settings.VelauxPassword, req.VelauxPassword)
	settings.LLMBaseURL = req.LLMBaseURL
	settings.LLMAPIKey = updateSecret(settings.LLMAPIKey, req.LLMAPIKey)
	settings.LLMModel = req.LLMModel
}

func validateSettingsURLs(ctx context.Context, policy sparkplugin.EndpointPolicy, req clusterSettingsRequest) error {
	for _, raw := range []string{req.PrometheusURL, req.VelauxURL, req.LLMBaseURL} {
		if raw == "" {
			continue
		}
		parsed, err := url.ParseRequestURI(raw)
		if err != nil || parsed.Host == "" || parsed.User != nil {
			return errInvalidSettingsURL
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			return errInvalidSettingsURL
		}
		if policy == nil || policy.Validate(ctx, raw) != nil {
			return errInvalidSettingsURL
		}
	}
	return nil
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	return maskedSecret
}

func updateSecret(existing, incoming string) string {
	if incoming == maskedSecret {
		return existing
	}
	return incoming
}
