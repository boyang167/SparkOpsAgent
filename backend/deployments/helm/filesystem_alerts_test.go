package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemAlertsCoverProductionFailureModes(t *testing.T) {
	alerts := readChartTemplate(t, "filesystem-alerts.yaml")
	for _, name := range []string{
		"FilesystemTransferFailureRateHigh",
		"FilesystemScannerUnavailable",
		"FilesystemProviderErrorRateHigh",
		"FilesystemQuotaReservationStuck",
		"FilesystemReconcileBacklogHigh",
		"FilesystemReplicaMismatch",
		"FilesystemOutboxLagHigh",
	} {
		require.Contains(t, alerts, "alert: "+name)
	}
	require.Contains(t, alerts, ".Values.observability.filesystemAlerts.enabled")
}
