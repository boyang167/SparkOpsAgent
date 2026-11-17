package model

import (
	"path/filepath"
	"strings"
)

type FilePreviewKind string

const (
	FilePreviewKindText        FilePreviewKind = "text"
	FilePreviewKindImage       FilePreviewKind = "image"
	FilePreviewKindPDF         FilePreviewKind = "pdf"
	FilePreviewKindUnsupported FilePreviewKind = "unsupported"
)

const (
	MaxFilePreviewTextBytes  int64 = 2 * 1024 * 1024
	MaxFilePreviewImageBytes int64 = 10 * 1024 * 1024
	MaxFilePreviewPDFBytes   int64 = 25 * 1024 * 1024
)

func ClassifyFilePreview(mimeType, fileName string) FilePreviewKind {
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	base := strings.ToLower(strings.TrimSpace(filepath.Base(fileName)))
	switch {
	case mime == "application/pdf" || strings.HasSuffix(base, ".pdf"):
		return FilePreviewKindPDF
	case strings.HasPrefix(mime, "image/"):
		return FilePreviewKindImage
	case isTextPreviewMIME(mime) || isTextPreviewExtension(base):
		return FilePreviewKindText
	default:
		return FilePreviewKindUnsupported
	}
}

func MaxPreviewBytes(kind FilePreviewKind) int64 {
	switch kind {
	case FilePreviewKindText:
		return MaxFilePreviewTextBytes
	case FilePreviewKindImage:
		return MaxFilePreviewImageBytes
	case FilePreviewKindPDF:
		return MaxFilePreviewPDFBytes
	default:
		return 0
	}
}

func isTextPreviewMIME(mime string) bool {
	switch mime {
	case "text/plain", "text/markdown", "text/yaml", "text/x-yaml", "text/csv", "text/xml",
		"text/html", "text/css", "text/javascript", "application/json", "application/xml",
		"application/yaml", "application/x-yaml", "application/javascript", "application/sql",
		"application/xhtml+xml":
		return true
	}
	return strings.HasPrefix(mime, "text/")
}

func isTextPreviewExtension(name string) bool {
	switch filepath.Ext(name) {
	case ".txt", ".md", ".markdown", ".yaml", ".yml", ".json", ".xml", ".csv", ".log",
		".ini", ".conf", ".properties", ".sh", ".bat", ".ps1", ".env", ".toml", ".sql", ".go",
		".ts", ".tsx", ".js", ".jsx", ".css", ".html", ".htm", ".cfg", ".cnf":
		return true
	}
	return false
}