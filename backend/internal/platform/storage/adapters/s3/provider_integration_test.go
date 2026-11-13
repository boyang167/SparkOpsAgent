//go:build integration

package s3

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/contracttest"
)

func TestS3ProviderIntegrationContract(t *testing.T) {
	provider := integrationProvider(t)
	contracttest.Run(t, provider)
}

func TestS3ProviderIntegrationMultipartContract(t *testing.T) {
	provider := integrationProvider(t)
	contracttest.RunMultipart(t, provider, provider)
}

func integrationProvider(t *testing.T) *Provider {
	t.Helper()
	if os.Getenv("REQUIRE_INTEGRATION") != "1" {
		t.Skip("set REQUIRE_INTEGRATION=1 to run S3 integration tests")
	}
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
	if endpoint == "" || bucket == "" {
		t.Skip("S3_ENDPOINT and S3_BUCKET required for integration tests (MinIO or compatible endpoint)")
	}
	cfg := Config{
		Endpoint:  endpoint,
		Region:    envOr("S3_REGION", "us-east-1"),
		Bucket:    bucket,
		PathStyle: os.Getenv("S3_PATH_STYLE") == "1" || os.Getenv("S3_PATH_STYLE") == "true",
		AccessKey: envOr("S3_ACCESS_KEY", "minioadmin"),
		SecretKey: envOr("S3_SECRET_KEY", "minioadmin"),
	}
	provider, err := New(cfg)
	require.NoError(t, err)
	if !bucketReachable(context.Background(), provider) {
		t.Skip("S3 bucket not reachable; start MinIO or set S3_* env vars")
	}
	return provider
}

func bucketReachable(ctx context.Context, p *Provider) bool {
	_, err := p.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(p.bucket)})
	return err == nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
