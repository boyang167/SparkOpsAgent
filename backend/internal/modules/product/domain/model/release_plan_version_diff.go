package model

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"time"
)

type ReleasePlanVersionOrderItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ReleasePlanVersionDiffChange struct {
	ChangeType  string                        `json:"change_type,omitempty"`
	Path        string                        `json:"path,omitempty"`
	Label       string                        `json:"label"`
	Before      any                           `json:"before,omitempty"`
	After       any                           `json:"after,omitempty"`
	BeforeOrder []ReleasePlanVersionOrderItem `json:"before_order,omitempty"`
	AfterOrder  []ReleasePlanVersionOrderItem `json:"after_order,omitempty"`
}

type ReleasePlanVersionDiffGroup struct {
	GroupKey  string                         `json:"group_key"`
	GroupName string                         `json:"group_name"`
	GroupType string                         `json:"group_type"`
	Changes   []ReleasePlanVersionDiffChange `json:"changes"`
}

type ReleasePlanVersionDiffResponse struct {
	PlanID          string                        `json:"plan_id"`
	Version         int                           `json:"version"`
	PreviousVersion int                           `json:"previous_version"`
	Groups          []ReleasePlanVersionDiffGroup `json:"groups"`
}

var releasePlanVersionFieldLabels = map[string]string{
	"name":                  "计划名称",
	"description":           "描述",
	"manager_id":            "发布负责人",
	"sprint_id":             "关联迭代",
	"status":                "状态",
	"release_window_start":  "发布窗口开始",
	"release_window_end":    "发布窗口结束",
	"schedule_execute_time": "定时执行",
	"approval.enabled":      "启用审批",
	"approval.provider":     "审批方式",
	"approval.approver_ids": "审批人",
}

func DiffReleasePlanVersionSnapshots(planID string, version, previousVersion int, before, after ReleasePlanVersionSnapshot) ReleasePlanVersionDiffResponse {
	resp := ReleasePlanVersionDiffResponse{
		PlanID: planID, Version: version, PreviousVersion: previousVersion,
		Groups: []ReleasePlanVersionDiffGroup{},
	}
	if previousVersion <= 0 {
		resp.Groups = append(resp.Groups, buildReleasePlanCreateDiffGroup(after))
		return resp
	}
	metaChanges := diffReleasePlanMetadata(before, after)
	if len(metaChanges) > 0 {
		resp.Groups = append(resp.Groups, ReleasePlanVersionDiffGroup{
			GroupKey: "plan", GroupName: "发布计划", GroupType: "plan", Changes: metaChanges,
		})
	}
	approvalChanges := diffReleasePlanApproval(before.ApprovalConfig, after.ApprovalConfig)
	if len(approvalChanges) > 0 {
		resp.Groups = append(resp.Groups, ReleasePlanVersionDiffGroup{
			GroupKey: "approval", GroupName: "审批配置", GroupType: "approval", Changes: approvalChanges,
		})
	}
	jobGroups := diffReleasePlanJobs(before.Jobs, after.Jobs)
	resp.Groups = append(resp.Groups, jobGroups...)
	return resp
}

func buildReleasePlanCreateDiffGroup(after ReleasePlanVersionSnapshot) ReleasePlanVersionDiffGroup {
	changes := diffReleasePlanMetadata(ReleasePlanVersionSnapshot{}, after)
	changes = append(changes, diffReleasePlanApproval(ApprovalConfig{}, after.ApprovalConfig)...)
	for _, job := range after.Jobs {
		changes = append(changes, ReleasePlanVersionDiffChange{
			ChangeType: "job_added",
			Path:       "jobs." + job.ID,
			Label:      "新增发布项 · " + job.Name,
			After:      summarizeReleasePlanJobSnapshot(job),
		})
	}
	return ReleasePlanVersionDiffGroup{
		GroupKey: "plan", GroupName: "发布计划", GroupType: "plan", Changes: changes,
	}
}

func diffReleasePlanMetadata(before, after ReleasePlanVersionSnapshot) []ReleasePlanVersionDiffChange {
	changes := []ReleasePlanVersionDiffChange{}
	appendField := func(path, label string, b, a any) {
		if reflect.DeepEqual(normalizeDiffValue(b), normalizeDiffValue(a)) {
			return
		}
		changes = append(changes, ReleasePlanVersionDiffChange{
			ChangeType: "field_changed", Path: path, Label: label, Before: b, After: a,
		})
	}
	appendField("name", releasePlanVersionFieldLabels["name"], before.Name, after.Name)
	appendField("description", releasePlanVersionFieldLabels["description"], before.Description, after.Description)
	appendField("manager_id", releasePlanVersionFieldLabels["manager_id"], before.ManagerID, after.ManagerID)
	appendField("sprint_id", releasePlanVersionFieldLabels["sprint_id"], before.SprintID, after.SprintID)
	appendField("status", releasePlanVersionFieldLabels["status"], string(before.Status), string(after.Status))
	appendField("release_window_start", releasePlanVersionFieldLabels["release_window_start"], formatDiffTime(before.ReleaseWindowStart), formatDiffTime(after.ReleaseWindowStart))
	appendField("release_window_end", releasePlanVersionFieldLabels["release_window_end"], formatDiffTime(before.ReleaseWindowEnd), formatDiffTime(after.ReleaseWindowEnd))
	appendField("schedule_execute_time", releasePlanVersionFieldLabels["schedule_execute_time"], formatDiffTimePtr(before.ScheduleExecuteTime), formatDiffTimePtr(after.ScheduleExecuteTime))
	for _, key := range unionParameterKeys(before.Parameters, after.Parameters) {
		b, a := before.Parameters[key], after.Parameters[key]
		if b == a {
			continue
		}
		changes = append(changes, ReleasePlanVersionDiffChange{
			ChangeType: "field_changed", Path: "parameters." + key, Label: "发布参数 · " + key,
			Before: parameterPresence(b, ""), After: parameterPresence(a, b),
		})
	}
	return changes
}

func unionParameterKeys(a, b map[string]string) []string {
	seen := map[string]struct{}{}
	keys := []string{}
	for _, m := range []map[string]string{a, b} {
		for key := range m {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// parameterPresence describes a parameter value without exposing it in version history.
func parameterPresence(value, previous string) string {
	switch {
	case value == "":
		return "未填写"
	case previous != "":
		return "已修改"
	default:
		return "已填写"
	}
}

func diffReleasePlanApproval(before, after ApprovalConfig) []ReleasePlanVersionDiffChange {
	changes := []ReleasePlanVersionDiffChange{}
	appendField := func(path, label string, b, a any) {
		if reflect.DeepEqual(normalizeDiffValue(b), normalizeDiffValue(a)) {
			return
		}
		changes = append(changes, ReleasePlanVersionDiffChange{
			ChangeType: "field_changed", Path: "approval." + path, Label: label, Before: b, After: a,
		})
	}
	appendField("enabled", releasePlanVersionFieldLabels["approval.enabled"], before.Enabled, after.Enabled)
	appendField("provider", releasePlanVersionFieldLabels["approval.provider"], string(before.ProviderOrDefault()), string(after.ProviderOrDefault()))
	appendField("approver_ids", releasePlanVersionFieldLabels["approval.approver_ids"], before.ApproverIDs, after.ApproverIDs)
	return changes
}

func diffReleasePlanJobs(before, after []ReleasePlanVersionJobSnapshot) []ReleasePlanVersionDiffGroup {
	beforeMap := map[string]ReleasePlanVersionJobSnapshot{}
	afterMap := map[string]ReleasePlanVersionJobSnapshot{}
	for _, job := range before {
		beforeMap[job.ID] = job
	}
	for _, job := range after {
		afterMap[job.ID] = job
	}

	groups := []ReleasePlanVersionDiffGroup{}
	orderChanged := !sameJobOrder(before, after)
	if orderChanged {
		groups = append(groups, ReleasePlanVersionDiffGroup{
			GroupKey: "jobs_order", GroupName: "发布项顺序", GroupType: "jobs_order",
			Changes: []ReleasePlanVersionDiffChange{{
				ChangeType:  "order_changed",
				Label:       "发布项顺序调整",
				BeforeOrder: jobOrderItems(before),
				AfterOrder:  jobOrderItems(after),
			}},
		})
	}

	for id, job := range afterMap {
		prev, ok := beforeMap[id]
		if !ok {
			groups = append(groups, ReleasePlanVersionDiffGroup{
				GroupKey: "job:" + id, GroupName: job.Name, GroupType: "job",
				Changes: []ReleasePlanVersionDiffChange{{
					ChangeType: "job_added", Path: "jobs." + id, Label: "新增发布项", After: summarizeReleasePlanJobSnapshot(job),
				}},
			})
			continue
		}
		jobChanges := diffReleasePlanJobSnapshot(prev, job)
		if len(jobChanges) > 0 {
			groups = append(groups, ReleasePlanVersionDiffGroup{
				GroupKey: "job:" + id, GroupName: job.Name, GroupType: "job", Changes: jobChanges,
			})
		}
	}
	for id, job := range beforeMap {
		if _, ok := afterMap[id]; ok {
			continue
		}
		groups = append(groups, ReleasePlanVersionDiffGroup{
			GroupKey: "job:" + id, GroupName: job.Name, GroupType: "job",
			Changes: []ReleasePlanVersionDiffChange{{
				ChangeType: "job_removed", Path: "jobs." + id, Label: "删除发布项", Before: summarizeReleasePlanJobSnapshot(job),
			}},
		})
	}
	return groups
}

func diffReleasePlanJobSnapshot(before, after ReleasePlanVersionJobSnapshot) []ReleasePlanVersionDiffChange {
	changes := []ReleasePlanVersionDiffChange{}
	appendField := func(path, label string, b, a any) {
		if reflect.DeepEqual(normalizeDiffValue(b), normalizeDiffValue(a)) {
			return
		}
		changes = append(changes, ReleasePlanVersionDiffChange{
			ChangeType: "field_changed", Path: path, Label: label, Before: b, After: a,
		})
	}
	appendField("name", "名称", before.Name, after.Name)
	appendField("type", "类型", string(before.Type), string(after.Type))
	appendField("owner_id", "执行人", before.OwnerID, after.OwnerID)
	appendField("text_content", "检查项内容", before.TextContent, after.TextContent)
	appendField("text_format", "内容格式", before.TextFormat, after.TextFormat)
	appendField("workflow_ref", "工作流配置", decodeJSONValue(before.WorkflowRef), decodeJSONValue(after.WorkflowRef))
	if !orderChangedOnly(before.SortOrder, after.SortOrder) && before.SortOrder != after.SortOrder {
		appendField("sort_order", "排序", before.SortOrder, after.SortOrder)
	}
	return changes
}

func summarizeReleasePlanJobSnapshot(job ReleasePlanVersionJobSnapshot) map[string]any {
	out := map[string]any{
		"name": job.Name, "type": string(job.Type), "sort_order": job.SortOrder,
	}
	if job.OwnerID != "" {
		out["owner_id"] = job.OwnerID
	}
	if job.TextContent != "" {
		out["text_content"] = job.TextContent
	}
	if len(job.WorkflowRef) > 0 {
		out["workflow_ref"] = decodeJSONValue(job.WorkflowRef)
	}
	return out
}

func jobOrderItems(jobs []ReleasePlanVersionJobSnapshot) []ReleasePlanVersionOrderItem {
	out := make([]ReleasePlanVersionOrderItem, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, ReleasePlanVersionOrderItem{ID: job.ID, Name: job.Name})
	}
	return out
}

func sameJobOrder(before, after []ReleasePlanVersionJobSnapshot) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		if before[i].ID != after[i].ID {
			return false
		}
	}
	return true
}

func orderChangedOnly(beforeOrder, afterOrder int) bool {
	return beforeOrder != afterOrder
}

func formatDiffTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func formatDiffTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func decodeJSONValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	return out
}

func normalizeDiffValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return strings.TrimSpace(typed)
	case *string:
		if typed == nil {
			return nil
		}
		return strings.TrimSpace(*typed)
	default:
		return value
	}
}
