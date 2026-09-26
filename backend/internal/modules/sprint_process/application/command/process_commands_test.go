package command_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type fakeProductChecker struct{ exists bool }

func (f fakeProductChecker) ProductExists(_ context.Context, _ kernel.ProductID) error {
	if f.exists {
		return nil
	}
	return domain.ErrProductNotFound
}

type fakeProcessRepo struct {
	defaultProcess *model.SprintProcess
	byID           map[kernel.SprintProcessID]*model.SprintProcess
	saved          []*model.SprintProcess
}

func (f *fakeProcessRepo) FindDefaultByProduct(_ context.Context, _ kernel.ProductID) (*model.SprintProcess, error) {
	if f.defaultProcess == nil {
		return nil, domain.ErrSprintProcessNotFound
	}
	return f.defaultProcess, nil
}

func (f *fakeProcessRepo) FindByID(_ context.Context, _ kernel.ProductID, id kernel.SprintProcessID) (*model.SprintProcess, error) {
	if p, ok := f.byID[id]; ok {
		return p, nil
	}
	return nil, domain.ErrSprintProcessNotFound
}

func (f *fakeProcessRepo) ListByProduct(_ context.Context, _ kernel.ProductID) ([]*model.SprintProcess, error) {
	return nil, nil
}

func (f *fakeProcessRepo) Save(_ context.Context, p *model.SprintProcess) error {
	f.saved = append(f.saved, p)
	f.defaultProcess = p
	f.byID = map[kernel.SprintProcessID]*model.SprintProcess{p.ID: p}
	return nil
}

func (f *fakeProcessRepo) Update(_ context.Context, p *model.SprintProcess) error {
	f.byID[p.ID] = p
	if p.IsDefault {
		f.defaultProcess = p
	}
	return nil
}

func (f *fakeProcessRepo) Delete(_ context.Context, _ kernel.ProductID, id kernel.SprintProcessID) error {
	delete(f.byID, id)
	return nil
}

type fakeUsageChecker struct{ count int }

func (f fakeUsageChecker) CountSprintsByProcess(_ context.Context, _ kernel.SprintProcessID) (int, error) {
	return f.count, nil
}

type fakeWorkspaceTemplateRepo struct {
	items map[kernel.WorkspaceSprintProcessTemplateID]*model.WorkspaceSprintProcessTemplate
}

func newFakeWorkspaceTemplateRepo() *fakeWorkspaceTemplateRepo {
	return &fakeWorkspaceTemplateRepo{items: map[kernel.WorkspaceSprintProcessTemplateID]*model.WorkspaceSprintProcessTemplate{}}
}

func (f *fakeWorkspaceTemplateRepo) List(_ context.Context, _ kernel.WorkspaceID) ([]*model.WorkspaceSprintProcessTemplate, error) {
	out := make([]*model.WorkspaceSprintProcessTemplate, 0, len(f.items))
	for _, item := range f.items {
		out = append(out, item)
	}
	return out, nil
}

func (f *fakeWorkspaceTemplateRepo) Save(_ context.Context, tpl *model.WorkspaceSprintProcessTemplate) error {
	f.items[tpl.ID] = tpl
	return nil
}

func (f *fakeWorkspaceTemplateRepo) Update(_ context.Context, tpl *model.WorkspaceSprintProcessTemplate) error {
	if _, ok := f.items[tpl.ID]; !ok {
		return domain.ErrWorkspaceSprintProcessTemplateNotFound
	}
	f.items[tpl.ID] = tpl
	return nil
}

func (f *fakeWorkspaceTemplateRepo) FindByID(_ context.Context, _ kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) (*model.WorkspaceSprintProcessTemplate, error) {
	if tpl, ok := f.items[id]; ok {
		return tpl, nil
	}
	return nil, domain.ErrWorkspaceSprintProcessTemplateNotFound
}

func (f *fakeWorkspaceTemplateRepo) Delete(_ context.Context, _ kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) error {
	if _, ok := f.items[id]; !ok {
		return domain.ErrWorkspaceSprintProcessTemplateNotFound
	}
	delete(f.items, id)
	return nil
}

var (
	_ repository.ProcessRepository           = (*fakeProcessRepo)(nil)
	_ repository.WorkspaceTemplateRepository = (*fakeWorkspaceTemplateRepo)(nil)
	_ port.ProductChecker                    = fakeProductChecker{}
	_ port.SprintUsageChecker                = fakeUsageChecker{}
)

func TestGetOrSeedSprintProcessHandler_SeedsWhenMissing(t *testing.T) {
	repo := &fakeProcessRepo{}
	products := fakeProductChecker{exists: true}
	h := command.NewGetOrSeedSprintProcessHandler(products, repo)

	productID := kernel.NewProductID()
	actor := kernel.NewMemberID()

	process, err := h.Handle(context.Background(), productID, actor)
	require.NoError(t, err)
	require.NotNil(t, process)
	require.True(t, process.IsDefault)
	require.Len(t, repo.saved, 1)
}

func TestDeleteSprintProcessHandler_RejectsDefault(t *testing.T) {
	productID := kernel.NewProductID()
	processID := kernel.NewSprintProcessID()
	process := model.NewSprintProcess(productID, "default", model.DefaultSprintStages(), kernel.NewMemberID())
	process.ID = processID
	process.IsDefault = true

	repo := &fakeProcessRepo{
		defaultProcess: process,
		byID:           map[kernel.SprintProcessID]*model.SprintProcess{processID: process},
	}
	h := command.NewDeleteSprintProcessHandler(fakeProductChecker{exists: true}, repo, fakeUsageChecker{})

	err := h.Handle(context.Background(), productID, processID)
	require.ErrorIs(t, err, domain.ErrSprintProcessDefaultProtected)
}

func TestDeleteSprintProcessHandler_RejectsInUse(t *testing.T) {
	productID := kernel.NewProductID()
	processID := kernel.NewSprintProcessID()
	process := model.NewSprintProcess(productID, "custom", model.DefaultSprintStages(), kernel.NewMemberID())
	process.ID = processID
	process.IsDefault = false

	repo := &fakeProcessRepo{byID: map[kernel.SprintProcessID]*model.SprintProcess{processID: process}}
	h := command.NewDeleteSprintProcessHandler(fakeProductChecker{exists: true}, repo, fakeUsageChecker{count: 2})

	err := h.Handle(context.Background(), productID, processID)
	require.ErrorIs(t, err, domain.ErrSprintProcessInUse)
}

func TestListWorkspaceSprintTemplatesHandler_SeedsWhenEmpty(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	actor := kernel.NewMemberID()
	repo := newFakeWorkspaceTemplateRepo()
	h := command.NewListWorkspaceSprintTemplatesHandler(repo)

	items, err := h.Handle(context.Background(), wsID, actor)
	require.NoError(t, err)
	require.Len(t, items, 3)
	keys := map[string]string{}
	for _, item := range items {
		keys[item.TemplateKey] = item.Name
	}
	require.Equal(t, "标准敏捷（4 列）", keys["agile-4"])
	require.Equal(t, "精简 Kanban（3 列）", keys["lean-3"])
	require.Equal(t, "DevOps 全流程（6 列）", keys["devops-6"])
}

func TestListWorkspaceSprintTemplatesHandler_BackfillsMissingBuiltins(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	actor := kernel.NewMemberID()
	repo := newFakeWorkspaceTemplateRepo()
	existing, err := model.NewWorkspaceSprintProcessTemplate(
		wsID, "标准敏捷（4 列）", "agile-4", "开发 → 测试 → 预发 → 发布", model.DefaultSprintStages(), actor,
	)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), existing))

	h := command.NewListWorkspaceSprintTemplatesHandler(repo)
	items, err := h.Handle(context.Background(), wsID, actor)
	require.NoError(t, err)
	require.Len(t, items, 3)
}

func TestUpdateWorkspaceSprintTemplateHandler_UpdatesNameAndStages(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	actor := kernel.NewMemberID()
	tpl, err := model.NewWorkspaceSprintProcessTemplate(wsID, "原始", "", "", model.DefaultSprintStages(), actor)
	require.NoError(t, err)

	repo := newFakeWorkspaceTemplateRepo()
	require.NoError(t, repo.Save(context.Background(), tpl))

	h := command.NewUpdateWorkspaceSprintTemplateHandler(repo)
	updated, err := h.Handle(context.Background(), wsID, tpl.ID, command.UpdateWorkspaceSprintTemplateInput{
		Name:   "更新后",
		Stages: model.DefaultSprintStages(),
	})
	require.NoError(t, err)
	require.Equal(t, "更新后", updated.Name)
}
