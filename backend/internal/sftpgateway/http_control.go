package sftpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HTTPTransferControl connects SFTP landing-zone imports to the API control plane.
type HTTPTransferControl struct {
	baseURL     string
	token       string
	storageRoot string
	client      *http.Client
	sessions    map[string]pendingSFTP
}

type pendingSFTP struct {
	sessionID  string
	objectKey  string
	storageRoot string
	localPath  string
}

func NewHTTPTransferControl(baseURL, token, storageRoot string) (*HTTPTransferControl, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%w: empty control base url", ErrControlUnavailable)
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("%w: empty internal token", ErrControlUnavailable)
	}
	return &HTTPTransferControl{
		baseURL: baseURL, token: token, storageRoot: strings.TrimSpace(storageRoot),
		client:   &http.Client{Timeout: 120 * time.Second},
		sessions: map[string]pendingSFTP{},
	}, nil
}

func (h *HTTPTransferControl) BeginUpload(ctx context.Context, req BeginUploadRequest) error {
	if h == nil {
		return ErrControlUnavailable
	}
	key := pendingKey(req.SessionPath, req.RelPath)
	var payload struct {
		Data struct {
			SessionID          string `json:"session_id"`
			TemporaryObjectKey string `json:"temporary_object_key"`
			StorageRoot        string `json:"storage_root"`
		} `json:"data"`
		Code int `json:"code"`
	}
	body, _ := json.Marshal(map[string]any{
		"organization_id": req.Principal.OrganizationID.String(),
		"workspace_id":    req.Principal.WorkspaceID.String(),
		"actor_id":        req.Principal.ActorID.String(),
		"rel_path":        req.RelPath,
		"session_path":    req.SessionPath,
		"size":            req.Size,
		"sha256":          req.SHA256,
	})
	if err := h.postJSON(ctx, h.baseURL+"/internal/filesystem/v1/sftp/begin-upload", body, &payload); err != nil {
		return err
	}
	root := payload.Data.StorageRoot
	if root == "" {
		root = h.storageRoot
	}
	h.sessions[key] = pendingSFTP{
		sessionID: payload.Data.SessionID, objectKey: payload.Data.TemporaryObjectKey,
		storageRoot: root,
		localPath:   filepath.Join(req.SessionPath, filepath.FromSlash(req.RelPath)),
	}
	return nil
}

func (h *HTTPTransferControl) CompleteUpload(ctx context.Context, req CompleteUploadRequest) error {
	if h == nil {
		return ErrControlUnavailable
	}
	key := pendingKey(req.SessionPath, req.RelPath)
	pending, ok := h.sessions[key]
	if !ok {
		return fmt.Errorf("%w: unknown sftp session", ErrControlUnavailable)
	}
	delete(h.sessions, key)
	dest := filepath.Join(pending.storageRoot, filepath.FromSlash(pending.objectKey))
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	src := pending.localPath
	if pending.localPath == "" {
		src = filepath.Join(req.SessionPath, filepath.FromSlash(req.RelPath))
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	notifyBody, _ := json.Marshal(map[string]any{"size": req.Size, "sha256": req.SHA256})
	url := fmt.Sprintf("%s/internal/filesystem/v1/transfers/%s/uploaded", h.baseURL, pending.sessionID)
	return h.postJSON(ctx, url, notifyBody, nil)
}

func (h *HTTPTransferControl) postJSON(ctx context.Context, url string, body []byte, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SparkOps-Internal-Token", h.token)
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sftp control %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func pendingKey(sessionPath, relPath string) string {
	return sessionPath + "\x00" + relPath
}