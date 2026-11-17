package model

import "testing"

func TestClassifyFilePreview(t *testing.T) {
	tests := []struct {
		mime string
		name string
		want FilePreviewKind
	}{
		{"application/pdf", "doc.bin", FilePreviewKindPDF},
		{"application/octet-stream", "readme.md", FilePreviewKindText},
		{"application/x-yaml", "kubeconfig.yaml", FilePreviewKindText},
		{"image/png", "a.png", FilePreviewKindImage},
		{"application/zip", "archive.zip", FilePreviewKindUnsupported},
	}
	for _, tt := range tests {
		if got := ClassifyFilePreview(tt.mime, tt.name); got != tt.want {
			t.Fatalf("ClassifyFilePreview(%q,%q) = %q, want %q", tt.mime, tt.name, got, tt.want)
		}
	}
}