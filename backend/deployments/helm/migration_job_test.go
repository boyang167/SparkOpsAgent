package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationHookBlocksRolloutAndUsesPackagedRunner(t *testing.T) {
	job := readChartTemplate(t, "migration-job.yaml")
	require.Contains(t, job, `"helm.sh/hook": pre-install,pre-upgrade`)
	require.Contains(t, job, `"helm.sh/hook-weight": "-10"`)
	require.Contains(t, job, `"helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded`)
	require.Contains(t, job, `image: "{{ .Values.api.image.repository }}:{{ .Values.api.image.tag }}"`)
	require.Contains(t, job, `command: ["/bin/sh", "-ec"]`)
	require.Contains(t, job, `/app/migrate-schema -if-unversioned`)
	require.Contains(t, job, `/app/migrate-sql -path /app/migrations -min-version 70`)
	require.Contains(t, job, `key: DATABASE_URL`)
	require.Contains(t, job, `restartPolicy: Never`)
	require.Contains(t, job, `serviceAccountName: {{ include "sparkops.serviceAccountName" . }}`)
	require.Contains(t, job, `{{- with .Values.global.imagePullSecrets }}`)
	require.Contains(t, job, `imagePullSecrets:`)
	require.Contains(t, job, `{{- toYaml . | nindent 8 }}`)

	secret := readChartTemplate(t, "migration-secret.yaml")
	require.Contains(t, secret, `"helm.sh/hook-weight": "-20"`)
	require.Contains(t, secret, `"helm.sh/hook-delete-policy": before-hook-creation`)
	require.NotContains(t, secret, `before-hook-creation,hook-succeeded`)
	require.Contains(t, secret, `DATABASE_URL: {{ .Values.secrets.databaseDsn | quote }}`)

	dockerfile, err := os.ReadFile(filepath.Join("..", "docker", "Dockerfile"))
	require.NoError(t, err)
	for _, binary := range []string{
		"server", "migrate-schema", "migrate-sql",
		"file-transfer-gateway", "file-worker", "sftp-gateway", "file-reconciler",
		"migrate-knowledge-inventory", "migrate-knowledge-precopy", "migrate-knowledge-switch",
	} {
		require.Contains(t, string(dockerfile), `-o /out/`+binary)
	}
	require.Contains(t, string(dockerfile), `COPY migrations ./migrations`)
}

func readChartTemplate(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "deployments", "helm", "sparkops", "templates", name))
	require.NoError(t, err)
	return string(data)
}
