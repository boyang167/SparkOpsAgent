package model

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	TriggerGit     = "git"
	TriggerWebhook = "webhook"
	TriggerCron    = "cron"

	GitEventPush          = "push"
	GitEventTag           = "tag"
	GitEventMergeRequest  = "merge_request"
)

var variableKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

type WorkflowVariable struct {
	ID           string `json:"id"`
	Key          string `json:"key"`
	Value        string `json:"value,omitempty"`
	Description  string `json:"description,omitempty"`
	RuntimeInput bool   `json:"runtime_input,omitempty"`
	Secret       bool   `json:"secret,omitempty"`
	VaultPath    string `json:"vault_path,omitempty"`
}

type WorkflowTrigger struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Enabled      *bool    `json:"enabled,omitempty"`
	ProjectID    string   `json:"project_id,omitempty"`
	Events       []string `json:"events,omitempty"`
	BranchFilter string   `json:"branch_filter,omitempty"`
	Token        string   `json:"token,omitempty"`
	Schedule     string   `json:"schedule,omitempty"`
}

func (t WorkflowTrigger) Active() bool {
	return t.Enabled == nil || *t.Enabled
}

type WorkflowAdvanced struct {
	Concurrency    int `json:"concurrency"`
	TimeoutMinutes int  `json:"timeout_minutes"`
	Debug          bool `json:"debug,omitempty"`
}

func DefaultAdvanced() WorkflowAdvanced {
	return WorkflowAdvanced{Concurrency: 1}
}

func NormalizeVariables(items []WorkflowVariable) ([]WorkflowVariable, error) {
	if len(items) == 0 {
		return []WorkflowVariable{}, nil
	}
	out := make([]WorkflowVariable, 0, len(items))
	seenID := map[string]struct{}{}
	seenKey := map[string]struct{}{}
	for _, item := range items {
		item.Key = strings.TrimSpace(item.Key)
		if item.Key == "" {
			continue
		}
		if !variableKeyPattern.MatchString(item.Key) {
			return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "变量名无效: "+item.Key)
		}
		if _, ok := seenKey[item.Key]; ok {
			return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "变量名重复: "+item.Key)
		}
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			item.ID = uuid.NewString()
		}
		if _, ok := seenID[item.ID]; ok {
			continue
		}
		seenID[item.ID] = struct{}{}
		seenKey[item.Key] = struct{}{}
		item.Value = strings.TrimSpace(item.Value)
		item.Description = strings.TrimSpace(item.Description)
		item.VaultPath = strings.Trim(strings.TrimSpace(item.VaultPath), "/")
		out = append(out, item)
	}
	return out, nil
}

func NormalizeTriggers(items []WorkflowTrigger) ([]WorkflowTrigger, error) {
	if len(items) == 0 {
		return []WorkflowTrigger{}, nil
	}
	out := make([]WorkflowTrigger, 0, len(items))
	seenID := map[string]struct{}{}
	seenToken := map[string]struct{}{}
	for _, item := range items {
		item.Type = strings.TrimSpace(item.Type)
		switch item.Type {
		case TriggerGit, TriggerWebhook, TriggerCron:
		default:
			continue
		}
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			item.ID = uuid.NewString()
		}
		if _, ok := seenID[item.ID]; ok {
			continue
		}
		item.ProjectID = strings.TrimSpace(item.ProjectID)
		item.BranchFilter = strings.TrimSpace(item.BranchFilter)
		item.Schedule = strings.TrimSpace(item.Schedule)
		item.Token = strings.TrimSpace(item.Token)
		switch item.Type {
		case TriggerGit:
			if item.ProjectID == "" {
				return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "Git 触发需要选择项目")
			}
			item.Events = uniqueGitEvents(item.Events)
			if len(item.Events) == 0 {
				item.Events = []string{GitEventPush}
			}
			if item.Token == "" {
				item.Token = strings.ReplaceAll(uuid.NewString(), "-", "")
			}
		case TriggerWebhook:
			item.Events = nil
			item.ProjectID = ""
			item.Schedule = ""
			item.BranchFilter = ""
			if item.Token == "" {
				item.Token = strings.ReplaceAll(uuid.NewString(), "-", "")
			}
		case TriggerCron:
			item.Events = nil
			item.ProjectID = ""
			item.Token = ""
			item.BranchFilter = ""
			if _, err := cron.ParseStandard(item.Schedule); err != nil {
				return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "定时表达式无效: "+item.Schedule)
			}
		}
		if item.Token != "" {
			if _, ok := seenToken[item.Token]; ok {
				return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "触发器 Token 重复")
			}
			seenToken[item.Token] = struct{}{}
		}
		seenID[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

func NormalizeAdvanced(item WorkflowAdvanced) WorkflowAdvanced {
	if item.Concurrency < 0 {
		item.Concurrency = 1
	}
	if item.TimeoutMinutes < 0 {
		item.TimeoutMinutes = 0
	}
	return item
}

func uniqueGitEvents(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		switch item {
		case GitEventPush, GitEventTag, GitEventMergeRequest:
		default:
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func FindActiveTrigger(items []WorkflowTrigger, kind, token string) *WorkflowTrigger {
	token = strings.TrimSpace(token)
	for i := range items {
		item := items[i]
		if item.Type != kind || !item.Active() || item.Token != token {
			continue
		}
		copied := item
		return &copied
	}
	return nil
}

func TriggerAcceptsEvent(item WorkflowTrigger, event string) bool {
	if len(item.Events) == 0 {
		return event == GitEventPush
	}
	for _, current := range item.Events {
		if current == event {
			return true
		}
	}
	return false
}

func FormatMissingVariables(keys []string) error {
	return kernel.NewDomainError("PRODUCT_WORKFLOW_VARIABLE_REQUIRED", "缺少运行时必填变量: "+strings.Join(keys, ", "))
}

const secretValuePrefix = "enc:v1:"
const maskedSecretValue = "******"

type ValueCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(encoded string) (string, error)
}

func SealVariables(box ValueCipher, previous, next []WorkflowVariable) ([]WorkflowVariable, error) {
	prev := map[string]WorkflowVariable{}
	for _, item := range previous {
		prev[item.Key] = item
	}
	for i := range next {
		item := next[i]
		old, hasOld := prev[item.Key]
		switch {
		case item.Secret && (item.Value == "" || item.Value == maskedSecretValue) && hasOld:
			item.Value = old.Value
			item.Secret = true
		case item.Secret && !strings.HasPrefix(item.Value, secretValuePrefix):
			if box == nil {
				return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "未配置变量加密密钥")
			}
			enc, err := box.Encrypt(item.Value)
			if err != nil {
				return nil, err
			}
			item.Value = secretValuePrefix + enc
		case !item.Secret && strings.HasPrefix(item.Value, secretValuePrefix) && box != nil:
			plain, err := box.Decrypt(strings.TrimPrefix(item.Value, secretValuePrefix))
			if err != nil {
				return nil, err
			}
			item.Value = plain
		case !item.Secret && (item.Value == "" || item.Value == maskedSecretValue) && hasOld && old.Secret:
			item.Secret = false
			if box == nil {
				item.Value = old.Value
				break
			}
			plain, err := box.Decrypt(strings.TrimPrefix(old.Value, secretValuePrefix))
			if err != nil {
				return nil, err
			}
			item.Value = plain
		}
		next[i] = item
	}
	return next, nil
}

func RevealVariables(box ValueCipher, items []WorkflowVariable) ([]WorkflowVariable, error) {
	out := append([]WorkflowVariable(nil), items...)
	for i := range out {
		if !strings.HasPrefix(out[i].Value, secretValuePrefix) {
			continue
		}
		if box == nil {
			return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "未配置变量加密密钥")
		}
		plain, err := box.Decrypt(strings.TrimPrefix(out[i].Value, secretValuePrefix))
		if err != nil {
			return nil, err
		}
		out[i].Value = plain
	}
	return out, nil
}

func MaskSecretVariables(items []WorkflowVariable) []WorkflowVariable {
	out := append([]WorkflowVariable(nil), items...)
	for i := range out {
		if out[i].Secret || strings.HasPrefix(out[i].Value, secretValuePrefix) {
			out[i].Secret = true
			out[i].Value = ""
		}
	}
	return out
}
