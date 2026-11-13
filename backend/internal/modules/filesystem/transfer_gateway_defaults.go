package filesystem

import "github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"

func transferGatewayDefaults(c *appctx.Container) (publicURL, secret, audience string) {
	audience = "sparkops-file-transfer"
	if c == nil || c.Config == nil {
		return "", "filesystem-transfer-dev", audience
	}
	tg := c.Config.Filesystem.TransferGateway
	publicURL = tg.PublicURL
	secret = tg.CapabilitySecret
	if secret == "" {
		secret = c.Config.JWT.Secret
	}
	if tg.CapabilityAudience != "" {
		audience = tg.CapabilityAudience
	}
	if secret == "" {
		secret = "filesystem-transfer-dev"
	}
	return publicURL, secret, audience
}
