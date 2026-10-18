package engine

import (
	"encoding/json"
	"path"
	"strings"
)

type GitHook struct {
	Event           string
	Ref             string
	RefType         string
	Commit          string
	HTTPURL         string
	Path            string
	MergeRequestIID int
}

func ParseGitLabHook(body []byte) (GitHook, bool, error) {
	var payload struct {
		ObjectKind  string `json:"object_kind"`
		Ref         string `json:"ref"`
		CheckoutSHA string `json:"checkout_sha"`
		After       string `json:"after"`
		Project     struct {
			PathWithNamespace string `json:"path_with_namespace"`
			GitHTTPURL        string `json:"git_http_url"`
			HTTPURL           string `json:"http_url"`
			WebURL            string `json:"web_url"`
		} `json:"project"`
		ObjectAttributes struct {
			IID          int    `json:"iid"`
			Action       string `json:"action"`
			SourceBranch string `json:"source_branch"`
			LastCommit   struct {
				ID string `json:"id"`
			} `json:"last_commit"`
		} `json:"object_attributes"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return GitHook{}, false, err
	}
	hook := GitHook{
		HTTPURL: firstNonEmpty(payload.Project.GitHTTPURL, payload.Project.HTTPURL, payload.Project.WebURL),
		Path:    strings.Trim(payload.Project.PathWithNamespace, "/"),
	}
	switch payload.ObjectKind {
	case "push":
		hook.Event = "push"
		hook.RefType = "branch"
		hook.Ref = strings.TrimPrefix(payload.Ref, "refs/heads/")
		hook.Commit = firstNonEmpty(payload.CheckoutSHA, payload.After)
	case "tag_push":
		hook.Event = "tag"
		hook.RefType = "tag"
		hook.Ref = strings.TrimPrefix(payload.Ref, "refs/tags/")
		hook.Commit = firstNonEmpty(payload.CheckoutSHA, payload.After)
	case "merge_request":
		switch payload.ObjectAttributes.Action {
		case "open", "update", "reopen":
		default:
			return GitHook{}, true, nil
		}
		hook.Event = "merge_request"
		hook.RefType = "branch"
		hook.Ref = strings.TrimSpace(payload.ObjectAttributes.SourceBranch)
		hook.Commit = payload.ObjectAttributes.LastCommit.ID
		hook.MergeRequestIID = payload.ObjectAttributes.IID
	default:
		return GitHook{}, true, nil
	}
	if hook.Ref == "" {
		return GitHook{}, true, nil
	}
	return hook, false, nil
}

func MatchRefFilter(filter, ref string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" || filter == "*" {
		return true
	}
	for _, part := range strings.Split(filter, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == ref {
			return true
		}
		if ok, err := path.Match(part, ref); err == nil && ok {
			return true
		}
	}
	return false
}

func NormalizeGitURL(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimSuffix(raw, ".git")
	raw = strings.TrimRight(raw, "/")
	raw = strings.TrimPrefix(raw, "ssh://")
	if strings.HasPrefix(raw, "git@") {
		raw = strings.TrimPrefix(raw, "git@")
		raw = strings.Replace(raw, ":", "/", 1)
	}
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return strings.Trim(raw, "/")
}

func GitURLMatches(projectURL, hookURL, hookPath string) bool {
	left := NormalizeGitURL(projectURL)
	if left == "" {
		return false
	}
	right := NormalizeGitURL(hookURL)
	if right != "" && (left == right || strings.HasSuffix(left, "/"+right) || strings.HasSuffix(right, "/"+left)) {
		return true
	}
	pathKey := NormalizeGitURL(hookPath)
	if pathKey == "" {
		return false
	}
	return left == pathKey || strings.HasSuffix(left, "/"+pathKey)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
