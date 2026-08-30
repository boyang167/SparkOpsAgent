package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const maxUpstreamBodyBytes int64 = 16 << 20

var sensitiveKeyRE = regexp.MustCompile(`(?i)(certificate|token|client-key|private-key|password)`)

func sanitizeErrorResponse(resp *http.Response) error {
	if resp == nil || resp.StatusCode < http.StatusBadRequest || resp.Body == nil {
		return nil
	}
	body, err := readLimited(resp.Body, maxUpstreamBodyBytes)
	_ = resp.Body.Close()
	if err != nil {
		body = []byte(`{"message":"upstream error body too large"}`)
	}
	body = sanitizeBody(body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if n > limit {
		return nil, errUpstreamBodyTooLarge
	}
	return buf.Bytes(), nil
}

func sanitizeBody(body []byte) []byte {
	var v any
	if json.Unmarshal(body, &v) == nil {
		redactJSON(&v)
		if out, err := json.Marshal(v); err == nil {
			return out
		}
	}
	return redactPlaintext(body)
}

func redactJSON(v *any) {
	switch typed := (*v).(type) {
	case map[string]any:
		for key, value := range typed {
			if sensitiveKeyRE.MatchString(key) {
				typed[key] = "redacted"
				continue
			}
			child := value
			redactJSON(&child)
			typed[key] = child
		}
	case []any:
		for i, value := range typed {
			child := value
			redactJSON(&child)
			typed[i] = child
		}
	case string:
		if sensitiveKeyRE.MatchString(typed) {
			*v = "redacted"
		}
	}
}

func redactPlaintext(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		for _, key := range []string{"certificate", "token", "client-key", "private-key", "password"} {
			if idx := strings.Index(lower, key); idx >= 0 {
				lines[i] = line[:idx+len(key)] + "=redacted"
				break
			}
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
