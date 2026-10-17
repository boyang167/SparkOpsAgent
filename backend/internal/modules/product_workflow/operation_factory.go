package product_workflow

import (
	"fmt"
	"net/http"

	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type OperationFactory struct {
	Transactor      postgres.Transactor
	Scheduler       workflowoutbound.CompletionScheduler
	WebhookResolver workflowoutbound.WebhookResolver
	WebhookClient   *http.Client
	MailSender      workflowoutbound.NotificationSender
	FeishuSender    workflowoutbound.NotificationSender
	GitLabReporter  workflowoutbound.GitLabStatusReporter
}

func (f OperationFactory) Validate() error {
	switch {
	case f.Transactor == nil:
		return fmt.Errorf("product_workflow operation transactor is required")
	case f.Scheduler == nil:
		return fmt.Errorf("product_workflow operation scheduler is required")
	case f.WebhookResolver == nil:
		return fmt.Errorf("product_workflow operation webhook resolver is required")
	case f.MailSender == nil:
		return fmt.Errorf("product_workflow operation mail sender is required")
	case f.FeishuSender == nil:
		return fmt.Errorf("product_workflow operation Feishu sender is required")
	case f.GitLabReporter == nil:
		return fmt.Errorf("product_workflow operation GitLab reporter is required")
	default:
		return nil
	}
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if registry == nil {
		return fmt.Errorf("product_workflow operation registry is required")
	}
	handlers := []struct {
		operationType string
		handler       messaging.OperationHandler
	}{
		{workflowoutbound.OperationTypeBuildTrigger, workflowoutbound.NewBuildCompletionHandler(f.Transactor, f.Scheduler)},
		{workflowoutbound.OperationTypeDeployTrigger, workflowoutbound.NewDeployCompletionHandler(f.Transactor, f.Scheduler)},
		{workflowoutbound.OperationTypeWebhookSend, workflowoutbound.NewWebhookHandler(f.WebhookResolver, f.WebhookClient)},
		{workflowoutbound.OperationTypeMailSend, workflowoutbound.NewNotificationHandler(f.MailSender)},
		{workflowoutbound.OperationTypeFeishuUserSend, workflowoutbound.NewNotificationHandler(f.FeishuSender)},
		{workflowoutbound.OperationTypeGitLabStatusReport, workflowoutbound.NewGitLabStatusHandler(f.GitLabReporter)},
	}
	for _, item := range handlers {
		if err := registry.Register(item.operationType, item.handler); err != nil {
			return fmt.Errorf("register product_workflow operation %s: %w", item.operationType, err)
		}
	}
	return nil
}
