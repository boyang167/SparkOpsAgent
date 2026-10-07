package model

import "time"

type ReleasePlanHookEvent string

const (
	ReleasePlanHookEventFinishPlanning ReleasePlanHookEvent = "finish_planning"
	ReleasePlanHookEventBeforeExecute  ReleasePlanHookEvent = "before_execute"
	ReleasePlanHookEventAllDone          ReleasePlanHookEvent = "all_done"
)

const (
	ReleasePlanStatusWaitFinishPlanningExternalCheck ReleasePlanStatus = "wait_for_finish_planning_external_check"
	ReleasePlanStatusWaitExecuteExternalCheck        ReleasePlanStatus = "wait_for_execute_external_check"
	ReleasePlanStatusWaitAllDoneExternalCheck        ReleasePlanStatus = "wait_for_all_done_external_check"
	ReleasePlanStatusFinishPlanningExternalFailed    ReleasePlanStatus = "finish_planning_external_check_failed"
	ReleasePlanStatusExecuteExternalFailed             ReleasePlanStatus = "execute_external_check_failed"
	ReleasePlanStatusAllDoneExternalFailed             ReleasePlanStatus = "all_done_external_check_failed"
)

type ReleasePlanHookEventConfig struct {
	Enabled     bool   `json:"enabled"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

type ReleasePlanHookSettings struct {
	Enabled bool                                          `json:"enabled"`
	Events  map[ReleasePlanHookEvent]ReleasePlanHookEventConfig `json:"events,omitempty"`
}

type ReleasePlanHookState struct {
	Event   ReleasePlanHookEvent `json:"event"`
	Token   string               `json:"token"`
	Status  string               `json:"status"`
	SentAt  *time.Time           `json:"sent_at,omitempty"`
	ActedAt *time.Time           `json:"acted_at,omitempty"`
	Message string               `json:"message,omitempty"`
}

func (s ReleasePlanHookSettings) EventEnabled(event ReleasePlanHookEvent) bool {
	if !s.Enabled {
		return false
	}
	if s.Events == nil {
		return false
	}
	cfg, ok := s.Events[event]
	return ok && cfg.Enabled && cfg.URL != ""
}

func (s ReleasePlanHookSettings) EventURL(event ReleasePlanHookEvent) string {
	if s.Events == nil {
		return ""
	}
	return s.Events[event].URL
}

func (p *ReleasePlan) InitHookWait(event ReleasePlanHookEvent, token string, now time.Time) {
	p.HookState = &ReleasePlanHookState{
		Event:  event,
		Token:  token,
		Status: "pending",
		SentAt: &now,
	}
	switch event {
	case ReleasePlanHookEventFinishPlanning:
		p.Status = ReleasePlanStatusWaitFinishPlanningExternalCheck
	case ReleasePlanHookEventBeforeExecute:
		p.Status = ReleasePlanStatusWaitExecuteExternalCheck
	case ReleasePlanHookEventAllDone:
		p.Status = ReleasePlanStatusWaitAllDoneExternalCheck
	}
	p.UpdatedAt = now
}

func (p *ReleasePlan) FailHookCheck(message string, now time.Time) {
	if p.HookState == nil {
		return
	}
	p.HookState.Status = "failed"
	p.HookState.ActedAt = &now
	p.HookState.Message = message
	switch p.HookState.Event {
	case ReleasePlanHookEventFinishPlanning:
		p.Status = ReleasePlanStatusFinishPlanningExternalFailed
	case ReleasePlanHookEventBeforeExecute:
		p.Status = ReleasePlanStatusExecuteExternalFailed
	case ReleasePlanHookEventAllDone:
		p.Status = ReleasePlanStatusAllDoneExternalFailed
	}
	p.UpdatedAt = now
}

func (p *ReleasePlan) PassHookCheck(now time.Time) ReleasePlanHookEvent {
	event := p.HookState.Event
	p.HookState.Status = "passed"
	p.HookState.ActedAt = &now
	p.UpdatedAt = now
	return event
}

func (p *ReleasePlan) ClearHookState() {
	p.HookState = nil
}
