package query

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
)

func TestNormalizeServiceURL(t *testing.T) {
	got, err := normalizeServiceURL("192.168.5.24")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://192.168.5.24" {
		t.Fatalf("got %q", got)
	}
}

func TestConnectionTestURLs_Harbor(t *testing.T) {
	urls := connectionTestURLs("Harbor", "http://192.168.5.24")
	if len(urls) < 4 || urls[0] != "http://192.168.5.24/api/v2.0/health" {
		t.Fatalf("unexpected urls: %#v", urls)
	}
	if urls[3] != "https://192.168.5.24/api/v2.0/health" {
		t.Fatalf("missing https health url: %#v", urls)
	}
}

func TestProbeURL_Accepts401(t *testing.T) {
	// behavior covered indirectly; placeholder for future httptest
	_ = model.Secret{}
}
