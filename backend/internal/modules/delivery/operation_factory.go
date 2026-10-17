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

func (f OperationFactory) Descriptors() ([]messaging.OperationDescriptor, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return []messaging.OperationDescriptor{
		{Type: deliveryoutbound.OperationTypeWebhookSend, Handler: deliveryoutbound.NewWebhookHandler(f.Webhooks, f.WebhookClient).Handle},
		{Type: deliveryoutbound.OperationTypeJenkinsCredentialSync, Handler: deliveryoutbound.NewCredentialSyncHandler(f.CredentialSync)},
		{Type: deliveryoutbound.OperationTypeDeployQueuePublish, Handler: deliveryoutbound.NewDeployQueueHandler(f.Deployments, f.Queue)},
	}, nil
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	descriptors, err := f.Descriptors()
	if err != nil {
		return err
	}
	return messaging.RegisterOperationDescriptors(registry, descriptors)
}
