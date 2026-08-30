package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	plugininstall "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

var uninstallPluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

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
	deletePaths, err := uninstallDeletePaths(record)
	if err != nil {
		return nil, err
	}
	branch := "plugin/uninstall-" + id
	view, err := h.publisher.LookupPullRequest(ctx, branch)
	if err == nil {
		if err := record.MarkUninstalling(view.URL); err != nil {
			return nil, err
		}
		if err := h.repo.SavePlugin(ctx, record); err != nil {
			return record, err
		}
		return record, nil
	}
	if !errors.Is(err, plugininstall.ErrPullRequestNotFound) {
		return nil, err
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

	prURL, err := h.publisher.Publish(ctx, plugininstall.PublishRequest{
		Branch:  branch,
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

func uninstallDeletePaths(record *model.PluginRecord) ([]string, error) {
	if !uninstallPluginIDPattern.MatchString(record.ID) {
		return nil, fmt.Errorf("unsafe plugin id: %q", record.ID)
	}
	backendRoot := path.Join("backend/internal/plugins", record.ID)
	deletePaths := make([]string, 0, len(record.InstalledFiles)+1)
	seen := make(map[string]struct{}, len(record.InstalledFiles)+1)
	for _, name := range record.InstalledFiles {
		if err := plugininstall.ValidateRepoRelativePath(name); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(name, backendRoot+"/") && !strings.HasPrefix(name, "fronted/apps/web/") {
			return nil, fmt.Errorf("installed file is outside plugin roots: %q", name)
		}
		if _, ok := seen[name]; !ok {
			deletePaths = append(deletePaths, name)
			seen[name] = struct{}{}
		}
	}
	if _, ok := seen[backendRoot]; !ok {
		deletePaths = append(deletePaths, backendRoot)
	}
	return deletePaths, nil
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
	switch record.Status {
	case model.PluginStatusMerged, model.PluginStatusUninstallMerged:
		return record, nil
	case model.PluginStatusUninstalling:
		err = record.MarkUninstallMerged()
	case model.PluginStatusPROpen:
		err = record.MarkMerged()
	default:
		return nil, model.ErrInvalidPluginStatusTransition
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
		if errors.Is(err, domain.ErrPluginNotFound) {
			hasReceipt, receiptErr := h.repo.HasUninstallReceipt(ctx, id)
			if receiptErr != nil {
				return nil, receiptErr
			}
			if hasReceipt {
				return nil, nil
			}
		}
		return nil, err
	}
	switch record.Status {
	case model.PluginStatusDeployed:
		return record, nil
	case model.PluginStatusUninstallMerged:
		if err := h.repo.CompleteUninstall(ctx, id, time.Now().UTC()); err != nil {
			if errors.Is(err, domain.ErrPluginNotFound) {
				hasReceipt, receiptErr := h.repo.HasUninstallReceipt(ctx, id)
				if receiptErr != nil {
					return nil, receiptErr
				}
				if hasReceipt {
					return nil, nil
				}
			}
			return nil, err
		}
		return nil, nil
	case model.PluginStatusMerged:
		if err := record.MarkDeployed(); err != nil {
			return nil, err
		}
	default:
		return nil, model.ErrInvalidPluginStatusTransition
	}
	if err := h.repo.SavePlugin(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}
