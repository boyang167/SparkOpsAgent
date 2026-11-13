package helm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFilesystemProductionValuesRequireTwoReplicas(t *testing.T) {
	values := readChartValues(t, "values-production.yaml")
	workloads := map[string]string{
		"fileTransferGateway": "file-transfer-gateway-deployment.yaml",
		"fileWorker":          "file-worker-deployment.yaml",
		"sftpGateway":         "sftp-gateway-deployment.yaml",
		"fileReconciler":      "file-reconciler-deployment.yaml",
	}
	for workload, templateName := range workloads {
		workloadValues, ok := values[workload].(map[string]any)
		require.True(t, ok, "%s values are missing", workload)
		replicas, ok := workloadValues["replicaCount"].(int)
		require.True(t, ok, "%s replicaCount is missing", workload)
		require.GreaterOrEqual(t, replicas, 2, "%s must have at least two production replicas", workload)
		require.Contains(t, readChartTemplate(t, templateName), "replicas: {{ .Values."+workload+".replicaCount }}")
		if autoscaling, ok := workloadValues["autoscaling"].(map[string]any); ok {
			minReplicas, ok := autoscaling["minReplicas"].(int)
			require.True(t, ok, "%s minReplicas is missing", workload)
			require.GreaterOrEqual(t, minReplicas, 2, "%s autoscaling floor", workload)
		}
	}

	networkPolicy := readChartTemplate(t, "file-networkpolicy.yaml")
	require.Contains(t, networkPolicy, `app.kubernetes.io/component: file-transfer-gateway`)
	require.Contains(t, networkPolicy, `include "sparkops.api.selectorLabels"`)
	require.Contains(t, networkPolicy, `include "sparkops.web.selectorLabels"`)
	require.Contains(t, networkPolicy, `include "sparkops.selectorLabels"`)
	require.Contains(t, networkPolicy, "port: 8082")
	require.Contains(t, networkPolicy, "port: 2022")
	require.Contains(t, networkPolicy, "file-worker")
	require.Contains(t, networkPolicy, "file-reconciler")
	require.Contains(t, networkPolicy, ".Values.filesystem.networkPolicy.dependencyPorts")
	require.NotContains(t, networkPolicy, "ingress-nginx")
}

func TestFilesystemWorkloadsUseRestrictedPodSecurity(t *testing.T) {
	for _, name := range []string{
		"file-transfer-gateway-deployment.yaml",
		"file-worker-deployment.yaml",
		"sftp-gateway-deployment.yaml",
		"file-reconciler-deployment.yaml",
	} {
		template := readChartTemplate(t, name)
		require.Contains(t, template, "runAsNonRoot: true", name)
		require.Contains(t, template, "readOnlyRootFilesystem: true", name)
		require.Contains(t, template, "drop:", name)
		require.Contains(t, template, "- ALL", name)
		require.Contains(t, template, "topologySpreadConstraints:", name)
		require.Contains(t, template, "resources:", name)
		require.Contains(t, template, "envFrom:", name)
		require.Contains(t, template, "secretRef:", name)
		require.Contains(t, template, ".Values.filesystem.providerSecretRefs", name)
		require.NotContains(t, template, ".Values.secrets.", name)
		require.Contains(t, template, "/app/bin/", name)
	}
	require.Contains(t, readChartTemplate(t, "file-transfer-gateway-deployment.yaml"), `printf "%.0f"`)
}

func TestFilesystemChartSecretCarriesCapabilityAndHostKey(t *testing.T) {
	secret := readChartTemplate(t, "secret.yaml")
	require.Contains(t, secret, "FILESYSTEM_TRANSFER_CAPABILITY_SECRET: {{ .Values.secrets.filesystemCapabilitySecret | quote }}")
	require.Contains(t, secret, "SPARKOPS_SFTP_HOST_KEY_PEM: {{ .Values.secrets.sftpHostKeyPem | quote }}")
}

func TestFilesystemPodDisruptionBudgetsCoverWorkloads(t *testing.T) {
	pdb := readChartTemplate(t, "file-pdb.yaml")
	require.Contains(t, pdb, "kind: PodDisruptionBudget")
	for _, component := range []string{"file-transfer-gateway", "file-worker", "sftp-gateway", "file-reconciler"} {
		require.Contains(t, pdb, component)
	}
}

func TestFilesystemDefaultValuesExposeDataPlane(t *testing.T) {
	values := readChartValues(t, "values.yaml")
	filesystem, ok := values["filesystem"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, filesystem["enabled"])
	require.Equal(t, 21474836480, filesystem["maxFileBytes"])
	refs, ok := filesystem["providerSecretRefs"].([]any)
	require.True(t, ok)
	require.Empty(t, refs)

	networkPolicy, ok := filesystem["networkPolicy"].(map[string]any)
	require.True(t, ok)
	ports, ok := networkPolicy["dependencyPorts"].([]any)
	require.True(t, ok)
	seen := map[int]bool{}
	for _, port := range ports {
		number, ok := port.(int)
		require.True(t, ok, "dependency port %#v", port)
		seen[number] = true
	}
	for _, port := range []int{53, 5432, 5672, 6379, 9000, 3310} {
		require.True(t, seen[port], "missing dependency port %d", port)
	}
}

func TestFilesystemValuesSchemaExists(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(chartDir(), "values.schema.json"))
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{
		"filesystem", "fileTransferGateway", "fileWorker", "sftpGateway", "fileReconciler",
	} {
		require.Contains(t, properties, key)
	}
}

func readChartValues(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(chartDir(), name))
	require.NoError(t, err)
	var values map[string]any
	require.NoError(t, yaml.Unmarshal(data, &values))
	return values
}

func chartDir() string {
	return filepath.Join("..", "..", "..", "deployments", "helm", "sparkops")
}
