package query

import (
	"context"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DeploymentView struct {
	ID              string  `json:"id"`
	WorkspaceID     string  `json:"workspace_id"`
	ProjectID       string  `json:"project_id"`
	ClusterID       string  `json:"cluster_id"`
	BuildID         *string `json:"build_id,omitempty"`
	GitRefType      string  `json:"git_ref_type"`
	GitRefValue     string  `json:"git_ref_value"`
	ImageTag        string  `json:"image_tag"`
	CrossCluster    bool    `json:"cross_cluster"`
	Status          string  `json:"status"`
	RunType         string  `json:"run_type,omitempty"`
	RollbackOfRunID *string `json:"rollback_of_run_id,omitempty"`
	CurrentStage    string  `json:"current_stage,omitempty"`
	ErrorMessage    string  `json:"error_message,omitempty"`
	CreatedBy       string  `json:"created_by"`
	CreatedAt       string  `json:"created_at"`
	StartedAt       *string `json:"started_at,omitempty"`
	FinishedAt      *string `json:"finished_at,omitempty"`
}

type deploymentPageReader interface {
	ListPageByProject(context.Context, kernel.WorkspaceID, kernel.ProjectID, int, int) (kernel.Page[*model.DeploymentRun], error)
}

type ListDeploymentsHandler struct {
	repo deploymentPageReader
}

func NewListDeploymentsHandler(repo deploymentPageReader) *ListDeploymentsHandler {
	return &ListDeploymentsHandler{repo: repo}
}

func (h *ListDeploymentsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (kernel.Page[DeploymentView], error) {
	page, err := h.repo.ListPageByProject(ctx, wsID, projectID, 1, 20)
	if err != nil {
		return kernel.Page[DeploymentView]{}, err
	}
	items := make([]DeploymentView, 0, len(page.Items))
	for _, run := range page.Items {
		items = append(items, ToDeploymentView(run))
	}
	return kernel.NewPage(items, page.Total, page.Page, page.PageSize), nil
}

type GetDeploymentHandler struct {
	repo repository.DeploymentRunRepository
}

func NewGetDeploymentHandler(repo repository.DeploymentRunRepository) *GetDeploymentHandler {
	return &GetDeploymentHandler{repo: repo}
}

func (h *GetDeploymentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, runID kernel.DeploymentRunID) (*DeploymentView, error) {
	run, err := h.repo.FindByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.WorkspaceID != wsID {
		return nil, domain.ErrDeploymentRunNotFound
	}
	view := ToDeploymentView(run)
	return &view, nil
}

type ClusterOption struct {
	ID         string `json:"id"`
	Name       string `json:"cluster_name"`
	Server     string `json:"server"`
	Available  bool   `json:"available"`
	IsBuildEnv bool   `json:"is_build_env"`
	PriorityID int    `json:"priority_id"`
	ReasonCode string `json:"reason_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type DeploymentOptions struct {
	Clusters []ClusterOption `json:"clusters"`
}

type DeploymentOptionsHandler struct {
	reader port.ProjectDeploymentReader
}

func NewDeploymentOptionsHandler(reader port.ProjectDeploymentReader) *DeploymentOptionsHandler {
	return &DeploymentOptionsHandler{reader: reader}
}

func isProjectLevelDeploymentError(err error) bool {
	domainErr, ok := kernel.IsDomainError(err)
	if !ok {
		return err != nil
	}
	switch domainErr.Code() {
	case "DELIVERY_DEPLOYMENT_CLUSTER_NOT_AUTHORIZED",
		"DELIVERY_KUBECONFIG_MISSING",
		"DELIVERY_DEPLOYMENT_CLUSTER_NOT_FOUND":
		return false
	default:
		return true
	}
}

func clusterOptionFromLoad(cluster port.DeploymentCluster, loadErr error) (ClusterOption, error) {
	option := ClusterOption{
		ID: cluster.ID.String(), Name: cluster.Name, Server: cluster.Server,
		IsBuildEnv: cluster.IsBuildEnv, PriorityID: cluster.PriorityID,
	}
	if loadErr == nil {
		option.Available = true
		return option, nil
	}
	if domainErr, ok := kernel.IsDomainError(loadErr); ok {
		option.ReasonCode = domainErr.Code()
		option.Reason = domainErr.Error()
		return option, nil
	}
	return ClusterOption{}, loadErr
}

func (h *DeploymentOptionsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (DeploymentOptions, error) {
	clusters, err := h.reader.ListAuthorizedClusters(ctx, wsID)
	if err != nil {
		return DeploymentOptions{}, err
	}
	if len(clusters) == 0 {
		return DeploymentOptions{Clusters: []ClusterOption{}}, nil
	}

	_, probeErr := h.reader.Load(ctx, wsID, projectID, clusters[0].ID)
	if probeErr != nil && isProjectLevelDeploymentError(probeErr) {
		options := DeploymentOptions{Clusters: make([]ClusterOption, 0, len(clusters))}
		for _, cluster := range clusters {
			option, err := clusterOptionFromLoad(cluster, probeErr)
			if err != nil {
				return DeploymentOptions{}, err
			}
			options.Clusters = append(options.Clusters, option)
		}
		return options, nil
	}

	options := DeploymentOptions{Clusters: make([]ClusterOption, len(clusters))}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for index, cluster := range clusters {
		wg.Add(1)
		go func(index int, cluster port.DeploymentCluster) {
			defer wg.Done()
			var loadErr error
			if index == 0 {
				loadErr = probeErr
			} else {
				_, loadErr = h.reader.Load(ctx, wsID, projectID, cluster.ID)
			}
			option, err := clusterOptionFromLoad(cluster, loadErr)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			options.Clusters[index] = option
		}(index, cluster)
	}
	wg.Wait()
	if firstErr != nil {
		return DeploymentOptions{}, firstErr
	}
	return options, nil
}

type StreamDeploymentHandler struct {
	repo repository.DeploymentRunRepository
	hub  *stream.Hub
}

func NewStreamDeploymentHandler(repo repository.DeploymentRunRepository, hub *stream.Hub) *StreamDeploymentHandler {
	return &StreamDeploymentHandler{repo: repo, hub: hub}
}

func (h *StreamDeploymentHandler) Subscribe(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	runID kernel.DeploymentRunID,
) (<-chan string, string, bool, func(), error) {
	run, err := h.repo.FindByID(ctx, runID)
	if err != nil {
		return nil, "", false, nil, err
	}
	if run.WorkspaceID != wsID {
		return nil, "", false, nil, domain.ErrDeploymentRunNotFound
	}
	live, _, _, unsubscribeLive := h.hub.Subscribe(runID)
	events, err := h.repo.ListEvents(ctx, runID)
	if err != nil {
		unsubscribeLive()
		return nil, "", false, nil, err
	}
	latest, err := h.repo.FindByID(ctx, runID)
	if err != nil {
		unsubscribeLive()
		return nil, "", false, nil, err
	}
	if latest.WorkspaceID != wsID {
		unsubscribeLive()
		return nil, "", false, nil, domain.ErrDeploymentRunNotFound
	}
	if isTerminalDeployment(latest.Status) {
		events, err = h.repo.ListEvents(ctx, runID)
		if err != nil {
			unsubscribeLive()
			return nil, "", false, nil, err
		}
		unsubscribeLive()
		ch := make(chan string)
		close(ch)
		return ch, stream.EncodeEvents(events), true, func() {}, nil
	}
	maxSnapshotID := uint64(0)
	for _, event := range events {
		if event.ID > maxSnapshotID {
			maxSnapshotID = event.ID
		}
	}
	output := make(chan string, 32)
	liveCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer close(output)
		for {
			select {
			case <-liveCtx.Done():
				return
			case delta, open := <-live:
				if !open {
					return
				}
				if eventID := stream.EventID(delta); eventID != 0 && eventID <= maxSnapshotID {
					continue
				}
				select {
				case output <- delta:
				case <-liveCtx.Done():
					return
				}
			}
		}
	}()
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			cancel()
			unsubscribeLive()
		})
	}
	return output, stream.EncodeEvents(events), false, unsubscribe, nil
}

func isTerminalDeployment(status model.Status) bool {
	return status == model.StatusSucceeded || status == model.StatusFailed || status == model.StatusAborted
}

func ToDeploymentView(run *model.DeploymentRun) DeploymentView {
	runType := string(run.RunType)
	if runType == "" {
		runType = string(model.RunTypeNormal)
	}
	view := DeploymentView{
		ID: run.ID.String(), WorkspaceID: run.WorkspaceID.String(), ProjectID: run.ProjectID.String(),
		ClusterID: run.ClusterID.String(), GitRefType: run.GitRefType, GitRefValue: run.GitRefValue,
		ImageTag: run.ImageTag, CrossCluster: run.CrossCluster, Status: string(run.Status), RunType: runType,
		CurrentStage: string(run.CurrentStage),
		ErrorMessage: run.ErrorMessage, CreatedBy: run.CreatedBy.String(), CreatedAt: run.CreatedAt.Format(time.RFC3339),
	}
	if run.RollbackOfRunID != nil {
		value := run.RollbackOfRunID.String()
		view.RollbackOfRunID = &value
	}
	if run.BuildID != nil {
		value := run.BuildID.String()
		view.BuildID = &value
	}
	if run.StartedAt != nil {
		value := run.StartedAt.Format(time.RFC3339)
		view.StartedAt = &value
	}
	if run.FinishedAt != nil {
		value := run.FinishedAt.Format(time.RFC3339)
		view.FinishedAt = &value
	}
	return view
}
