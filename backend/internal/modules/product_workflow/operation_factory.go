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

func (f OperationFactory) Descriptors() ([]messaging.OperationDescriptor, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return []messaging.OperationDescriptor{
		{Type: workflowoutbound.OperationTypeBuildTrigger, Handler: workflowoutbound.NewBuildCompletionHandler(f.Transactor, f.Scheduler)},
		{Type: workflowoutbound.OperationTypeDeployTrigger, Handler: workflowoutbound.NewDeployCompletionHandler(f.Transactor, f.Scheduler)},
		{Type: workflowoutbound.OperationTypeWebhookSend, Handler: workflowoutbound.NewWebhookHandler(f.WebhookResolver, f.WebhookClient)},
		{Type: workflowoutbound.OperationTypeMailSend, Handler: workflowoutbound.NewNotificationHandler(f.MailSender)},
		{Type: workflowoutbound.OperationTypeFeishuUserSend, Handler: workflowoutbound.NewNotificationHandler(f.FeishuSender)},
		{Type: workflowoutbound.OperationTypeGitLabStatusReport, Handler: workflowoutbound.NewGitLabStatusHandler(f.GitLabReporter)},
	}, nil
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	descriptors, err := f.Descriptors()
	if err != nil {
		return err
	}
	return messaging.RegisterOperationDescriptors(registry, descriptors)
}
