package command

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	plugininstall "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UninstallHandler struct {
	repo      repository.PluginRepository
	publisher plugininstall.GitPublisher
	root      string
}

func NewUninstallHandler(repo repository.PluginRepository, publisher plugininstall.GitPublisher, root ...string) *UninstallHandler {
	return &UninstallHandler{repo: repo, publisher: publisher, root: firstRoot(root)}
}

func (h *UninstallHandler) Handle(ctx context.Context, id string, actor kernel.MemberID) (*model.PluginRecord, error) {
	_ = actor
	record, err := h.repo.FindPlugin(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.Status != model.PluginStatusDeployed {
		return nil, domain.ErrPluginNotDeployed
	}
	bindings, err := h.repo.ListBindingsByPlugin(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		if binding.Enabled {
			return nil, domain.ErrPluginInUse
		}
	}

	root, err := resolveInstallRoot(h.root)
	if err != nil {
		return nil, err
	}
	currentRegister, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(registerGoPath)))
	if err != nil {
		return nil, fmt.Errorf("read plugin register: %w", err)
	}
	pluginIDs, err := registeredPluginIDs(currentRegister)
	if err != nil {
		return nil, fmt.Errorf("parse plugin register: %w", err)
	}
	kept := make([]string, 0, len(pluginIDs))
	for _, pluginID := range pluginIDs {
		if pluginID != id {
			kept = append(kept, pluginID)
		}
	}
	nextRegister, err := plugininstall.RewriteRegisterGo(currentRegister, backendModulePath, kept)
	if err != nil {
		return nil, err
	}

	storedManifest, err := manifest.Parse([]byte(record.ManifestYAML))
	if err != nil {
		return nil, err
	}
	deletePaths := []string{path.Join("backend/internal/plugins", id)}
	for _, page := range storedManifest.Frontend.Pages {
		deletePaths = append(deletePaths, path.Join("fronted/apps/web", page))
	}
	prURL, err := h.publisher.Publish(ctx, plugininstall.PublishRequest{
		Branch:  "plugin/uninstall-" + id,
		Message: "Uninstall plugin " + id,
		Files:   map[string][]byte{registerGoPath: nextRegister},
		Delete:  deletePaths,
	})
	if err != nil {
		return nil, err
	}
	if err := record.MarkUninstalling(prURL); err != nil {
		return nil, err
	}
	if err := h.repo.SavePlugin(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

type MarkMergedHandler struct {
	repo repository.PluginRepository
}

func NewMarkMergedHandler(repo repository.PluginRepository) *MarkMergedHandler {
	return &MarkMergedHandler{repo: repo}
}

func (h *MarkMergedHandler) Handle(ctx context.Context, id string) (*model.PluginRecord, error) {
	record, err := h.repo.FindPlugin(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.Status == model.PluginStatusUninstalling {
		err = record.MarkUninstallMerged()
	} else {
		err = record.MarkMerged()
	}
	if err != nil {
		return nil, err
	}
	if err := h.repo.SavePlugin(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

type MarkDeployedHandler struct {
	repo repository.PluginRepository
}

func NewMarkDeployedHandler(repo repository.PluginRepository) *MarkDeployedHandler {
	return &MarkDeployedHandler{repo: repo}
}

func (h *MarkDeployedHandler) Handle(ctx context.Context, id string) (*model.PluginRecord, error) {
	record, err := h.repo.FindPlugin(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.Status == model.PluginStatusUninstallMerged {
		if err := h.repo.DeletePlugin(ctx, id); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err := record.MarkDeployed(); err != nil {
		return nil, err
	}
	if err := h.repo.SavePlugin(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}
