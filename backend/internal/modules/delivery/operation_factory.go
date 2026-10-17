package delivery

import (
	"fmt"
	"net/http"

	deployrepo "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	deliveryoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

type OperationFactory struct {
	Webhooks       deliveryoutbound.WebhookLookup
	WebhookClient  *http.Client
	CredentialSync deliveryoutbound.CredentialSyncer
	Deployments    deployrepo.DeploymentRunRepository
	Queue          messaging.Queue
}

func (f OperationFactory) Validate() error {
	switch {
	case f.Webhooks == nil:
		return fmt.Errorf("delivery operation webhook lookup is required")
	case f.CredentialSync == nil:
		return fmt.Errorf("delivery operation credential sync is required")
	case f.Deployments == nil:
		return fmt.Errorf("delivery operation deployment repository is required")
	case f.Queue == nil:
		return fmt.Errorf("delivery operation queue is required")
	default:
		return nil
	}
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if registry == nil {
		return fmt.Errorf("delivery operation registry is required")
	}
	handlers := []struct {
		operationType string
		handler       messaging.OperationHandler
	}{
		{deliveryoutbound.OperationTypeWebhookSend, deliveryoutbound.NewWebhookHandler(f.Webhooks, f.WebhookClient).Handle},
		{deliveryoutbound.OperationTypeJenkinsCredentialSync, deliveryoutbound.NewCredentialSyncHandler(f.CredentialSync)},
		{deliveryoutbound.OperationTypeDeployQueuePublish, deliveryoutbound.NewDeployQueueHandler(f.Deployments, f.Queue)},
	}
	for _, item := range handlers {
		if err := registry.Register(item.operationType, item.handler); err != nil {
			return fmt.Errorf("register delivery operation %s: %w", item.operationType, err)
		}
	}
	return nil
}
