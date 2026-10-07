package persistence

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type apiModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	Path        string `gorm:"uniqueIndex:ux_platform_apis_path_method"`
	Method      string `gorm:"uniqueIndex:ux_platform_apis_path_method"`
	APIGroup    string `gorm:"column:api_group"`
	Description string
	postgres.AuditColumns
}

func (apiModel) TableName() string { return "platform.apis" }

type apiIgnoreModel struct {
	Path   string `gorm:"primaryKey"`
	Method string `gorm:"primaryKey"`
}

func (apiIgnoreModel) TableName() string { return "platform.api_ignore" }

type RegisteredAPI struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	APIGroup    string `json:"api_group"`
	Description string `json:"description"`
}

type APISyncDiff struct {
	New    []RegisteredAPI `json:"new"`
	Delete []RegisteredAPI `json:"delete"`
	Ignore []RegisteredAPI `json:"ignore"`
}

type APIRegistryStore struct {
	db *gorm.DB
}

func NewAPIRegistryStore(db *gorm.DB) *APIRegistryStore {
	return &APIRegistryStore{db: db}
}

func (s *APIRegistryStore) AutoMigrate() error {
	if !postgres.ShouldRunAutoMigrate(s.db) {
		return s.db.Exec(`
			CREATE UNIQUE INDEX IF NOT EXISTS ux_platform_apis_path_method
			ON platform.apis (path, method)
		`).Error
	}
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS platform").Error; err != nil {
			return err
		}
	}
	if err := s.db.AutoMigrate(&apiModel{}, &apiIgnoreModel{}); err != nil {
		return err
	}
	// GORM may have created platform.apis before migration 000018; ensure binder ON CONFLICT works.
	return s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS ux_platform_apis_path_method
		ON platform.apis (path, method)
	`).Error
}

func (s *APIRegistryStore) ListAll(ctx context.Context) ([]RegisteredAPI, error) {
	var rows []apiModel
	if err := s.db.WithContext(ctx).Order("api_group asc, path asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToAPIs(rows), nil
}

func (s *APIRegistryStore) Diff(
	ctx context.Context,
	routes []struct{ Method, Path string },
) (APISyncDiff, error) {
	existing, err := s.ListAll(ctx)
	if err != nil {
		return APISyncDiff{}, err
	}
	ignore, err := s.listIgnore(ctx)
	if err != nil {
		return APISyncDiff{}, err
	}
	existingMap := map[string]RegisteredAPI{}
	for _, item := range existing {
		existingMap[key(item.Method, item.Path)] = item
	}
	ignoreMap := map[string]struct{}{}
	for _, item := range ignore {
		ignoreMap[key(item.Method, item.Path)] = struct{}{}
	}
	routeMap := map[string]struct{}{}
	diff := APISyncDiff{
		New:    []RegisteredAPI{},
		Delete: []RegisteredAPI{},
		Ignore: []RegisteredAPI{},
	}
	for _, route := range routes {
		k := key(route.Method, route.Path)
		routeMap[k] = struct{}{}
		if _, ok := ignoreMap[k]; ok {
			if item, exists := existingMap[k]; exists {
				diff.Ignore = append(diff.Ignore, item)
			}
			continue
		}
		if _, ok := existingMap[k]; !ok {
			group, desc := inferAPIMeta(route.Path, route.Method)
			diff.New = append(diff.New, RegisteredAPI{
				ID: uuid.NewString(), Path: route.Path, Method: route.Method,
				APIGroup: group, Description: desc,
			})
		}
	}
	for _, item := range existing {
		if _, ok := routeMap[key(item.Method, item.Path)]; !ok {
			diff.Delete = append(diff.Delete, item)
		}
	}
	return diff, nil
}

func (s *APIRegistryStore) ApplySync(ctx context.Context, diff APISyncDiff) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range diff.New {
			row := apiModel{
				ID: item.ID, Path: item.Path, Method: item.Method,
				APIGroup: item.APIGroup, Description: item.Description,
				AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		for _, item := range diff.Delete {
			if err := tx.Where("path = ? AND method = ?", item.Path, item.Method).Delete(&apiModel{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *APIRegistryStore) UpdateMeta(ctx context.Context, id string, group, description string) error {
	return s.db.WithContext(ctx).Model(&apiModel{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"api_group": group, "description": description, "updated_at": time.Now().UTC(),
		}).Error
}

func (s *APIRegistryStore) DeleteByID(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&apiModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *APIRegistryStore) DeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Where("id IN ?", ids).Delete(&apiModel{}).Error
}

func (s *APIRegistryStore) listIgnore(ctx context.Context) ([]RegisteredAPI, error) {
	var rows []apiIgnoreModel
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]RegisteredAPI, 0, len(rows))
	for _, row := range rows {
		out = append(out, RegisteredAPI{Path: row.Path, Method: row.Method})
	}
	return out, nil
}

func rowsToAPIs(rows []apiModel) []RegisteredAPI {
	out := make([]RegisteredAPI, 0, len(rows))
	for _, row := range rows {
		out = append(out, RegisteredAPI{
			ID: row.ID, Path: row.Path, Method: row.Method,
			APIGroup: row.APIGroup, Description: row.Description,
		})
	}
	return out
}

func key(method, path string) string { return method + " " + path }

func inferAPIMeta(path, method string) (group, description string) {
	path = strings.Split(path, "?")[0]
	path = strings.TrimRight(path, "/")
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 2 || segments[0] != "api" || segments[1] != "v1" {
		return "default", methodActionLabel(method)
	}
	rest := segments[2:]
	action := methodActionLabel(method)

	if len(rest) >= 2 && rest[0] == "plugins" {
		pluginID := rest[1]
		tail := firstLiteralSegment(rest[2:])
		group = "plugin:" + pluginID
		if tail != "" {
			description = humanizeSegment(tail) + " · " + action
		} else {
			description = pluginID + " · " + action
		}
		return group, description
	}

	if len(rest) >= 3 && rest[0] == "workspaces" {
		resource := rest[2]
		tail := firstLiteralSegment(rest[3:])
		group = "governance:" + resource
		if tail != "" {
			description = humanizeSegment(resource) + " · " + humanizeSegment(tail) + " · " + action
		} else {
			description = humanizeSegment(resource) + " · " + action
		}
		return group, description
	}

	if len(rest) >= 3 && rest[0] == "projects" {
		tail := literalSegments(rest[2:])
		if len(tail) >= 2 {
			group = tail[0] + ":" + tail[1]
			description = humanizeSegment(tail[0]) + " · " + humanizeSegment(tail[1]) + " · " + action
			return group, description
		}
		if len(tail) == 1 {
			group = "project:" + tail[0]
			description = humanizeSegment(tail[0]) + " · " + action
			return group, description
		}
	}

	if len(rest) >= 3 && rest[0] == "products" {
		resource := rest[2]
		tail := firstLiteralSegment(rest[3:])
		group = "product:" + resource
		if tail != "" {
			description = "产品 · " + humanizeSegment(resource) + " · " + humanizeSegment(tail) + " · " + action
		} else {
			description = "产品 · " + humanizeSegment(resource) + " · " + action
		}
		return group, description
	}

	if len(rest) >= 3 && rest[0] == "orgs" {
		resource := rest[2]
		group = "tenant:" + resource
		description = "组织 · " + humanizeSegment(resource) + " · " + action
		return group, description
	}

	if len(rest) >= 1 {
		module := rest[0]
		tail := firstLiteralSegment(rest[1:])
		group = module
		if tail != "" {
			description = humanizeSegment(module) + " · " + humanizeSegment(tail) + " · " + action
		} else {
			description = humanizeSegment(module) + " · " + action
		}
		return group, description
	}

	return "default", action
}

func methodActionLabel(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "查询"
	case "POST":
		return "创建"
	case "PUT":
		return "更新"
	case "PATCH":
		return "部分更新"
	case "DELETE":
		return "删除"
	case "ANY":
		return "代理"
	default:
		return strings.ToUpper(method)
	}
}

func firstLiteralSegment(segments []string) string {
	for _, segment := range segments {
		if segment != "" && !strings.HasPrefix(segment, ":") {
			return segment
		}
	}
	return ""
}

func literalSegments(segments []string) []string {
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" && !strings.HasPrefix(segment, ":") {
			out = append(out, segment)
		}
	}
	return out
}

func humanizeSegment(segment string) string {
	labels := map[string]string{
		"members": "成员", "roles": "角色", "menus": "菜单", "apis": "API",
		"projects": "项目", "products": "产品", "builds": "构建", "runtime": "运行时",
		"deployment": "部署", "orgs": "组织", "workspaces": "工作区", "notifications": "通知",
		"auth": "认证", "plugins": "插件", "kubernetes": "Kubernetes",
	}
	if label, ok := labels[segment]; ok {
		return label
	}
	return strings.ReplaceAll(segment, "-", " ")
}
