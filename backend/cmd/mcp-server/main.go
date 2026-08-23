package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	apiURL := strings.TrimRight(envOr("SPARKOPS_API_URL", "http://localhost:8080"), "/")
	token := os.Getenv("SPARKOPS_TOKEN")
	workspaceID := os.Getenv("SPARKOPS_WORKSPACE_ID")
	if token == "" || workspaceID == "" {
		fmt.Fprintln(os.Stderr, "SPARKOPS_TOKEN and SPARKOPS_WORKSPACE_ID are required")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req rpcEnvelope
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeResp(rpcEnvelope{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}
		if req.JSONRPC == "" {
			req.JSONRPC = "2.0"
		}

		resp, err := forwardRPC(client, apiURL, token, workspaceID, req)
		if err != nil {
			writeResp(rpcEnvelope{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32000, Message: err.Error()},
			})
			continue
		}
		writeResp(resp)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "stdin read error:", err)
		os.Exit(1)
	}
}

func forwardRPC(client *http.Client, apiURL, token, workspaceID string, req rpcEnvelope) (rpcEnvelope, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return rpcEnvelope{}, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, apiURL+"/api/v1/mcp/rpc", bytes.NewReader(body))
	if err != nil {
		return rpcEnvelope{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-Workspace-Id", workspaceID)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return rpcEnvelope{}, err
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return rpcEnvelope{}, err
	}
	if httpResp.StatusCode == http.StatusUnauthorized {
		return rpcEnvelope{}, fmt.Errorf("unauthorized (check SPARKOPS_TOKEN)")
	}
	if httpResp.StatusCode >= 400 {
		return rpcEnvelope{}, fmt.Errorf("upstream HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out rpcEnvelope
	if err := json.Unmarshal(raw, &out); err != nil {
		return rpcEnvelope{}, err
	}
	return out, nil
}

func writeResp(resp rpcEnvelope) {
	if resp.JSONRPC == "" {
		resp.JSONRPC = "2.0"
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
