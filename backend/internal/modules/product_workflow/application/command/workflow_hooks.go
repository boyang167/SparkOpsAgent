package command

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func (h *StartWorkflowRunHandler) UseGitLookup(lookup ProjectGitLookup) {
	h.gitLookup = lookup
}

func (h *StartWorkflowRunHandler) StartFromWebhook(
	ctx context.Context,
	productID kernel.ProductID,
	workflowID kernel.ProductWorkflowID,
	token string,
	input map[string]any,
) (*model.ProductWorkflowRun, bool, error) {
	wf, err := h.workflows.FindWorkflowByID(ctx, workflowID)
	if err != nil {
		return nil, false, err
	}
	if wf.ProductID != productID {
		return nil, false, domain.ErrProductWorkflowNotFound
	}
	trigger := matchToken(wf.Triggers, model.TriggerWebhook, token)
	if trigger == nil {
		return nil, false, domain.ErrWorkflowHookRejected
	}
	if !trigger.Active() || wf.Status != model.WorkflowStatusActive {
		return nil, true, nil
	}
	run, err := h.launch(ctx, wf, "webhook", input, nil)
	return run, false, err
}

func (h *StartWorkflowRunHandler) StartFromGitLab(ctx context.Context, token string, body []byte) (*model.ProductWorkflowRun, bool, error) {
	hook, skip, err := engine.ParseGitLabHook(body)
	if err != nil {
		return nil, false, domain.ErrInvalidWorkflowInput
	}
	if skip {
		return nil, true, nil
	}
	items, err := h.workflows.ListAllWorkflows(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, wf := range items {
		if wf.Status != model.WorkflowStatusActive {
			continue
		}
		for _, trigger := range wf.Triggers {
			if trigger.Type != model.TriggerGit || !trigger.Active() || !tokenEqual(trigger.Token, token) {
				continue
			}
			if !model.TriggerAcceptsEvent(trigger, hook.Event) || !engine.MatchRefFilter(trigger.BranchFilter, hook.Ref) {
				continue
			}
			if h.gitLookup == nil {
				continue
			}
			projectID, err := kernel.ParseProjectID(trigger.ProjectID)
			if err != nil {
				continue
			}
			project, err := h.gitLookup.LookupProjectGit(ctx, projectID)
			if err != nil || project.ProductID != wf.ProductID {
				continue
			}
			if !engine.GitURLMatches(project.GitURL, hook.HTTPURL, hook.Path) {
				continue
			}
			run, err := h.launch(ctx, wf, "git", nil, &engine.GitRunContext{
				ProjectID: trigger.ProjectID,
				Event:     hook.Event,
				Ref:       hook.Ref,
				RefType:   hook.RefType,
				Commit:    hook.Commit,
			})
			return run, false, err
		}
	}
	return nil, true, nil
}

func (h *StartWorkflowRunHandler) StartFromCron(ctx context.Context, wf *model.ProductWorkflow) (*model.ProductWorkflowRun, error) {
	if wf == nil || wf.Status != model.WorkflowStatusActive {
		return nil, domain.ErrInvalidWorkflowInput
	}
	return h.launch(ctx, wf, "cron", nil, nil)
}

func matchToken(items []model.WorkflowTrigger, kind, token string) *model.WorkflowTrigger {
	token = strings.TrimSpace(token)
	for i := range items {
		item := items[i]
		if item.Type != kind || !tokenEqual(item.Token, token) {
			continue
		}
		copied := item
		return &copied
	}
	return nil
}

func tokenEqual(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
