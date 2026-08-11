package model

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Menu struct {
	ID                  string
	WorkspaceID         kernel.WorkspaceID
	ParentID            *string
	Name                string
	Path                string
	Title               string
	Icon                string
	Component           string
	Hidden              bool
	Sort                int
	KeepAlive           bool
	ActiveMenu          string
	CloseTab            bool
	IsBasic             bool
	TransitionAnimation string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Params              []MenuParam
	Buttons             []MenuButton
}

type MenuParam struct {
	ID        string
	MenuID    string
	Type      string
	Key       string
	Value     string
	Sort      int
}

type MenuButton struct {
	ID      string
	MenuID  string
	Name    string
	Code    string
	Remark  string
	Sort    int
	RoleIDs []string
}

type MenuTreeNode struct {
	ID                  string         `json:"id"`
	ParentID            *string        `json:"parent_id,omitempty"`
	Name                string         `json:"name"`
	Path                string         `json:"path"`
	Title               string         `json:"title"`
	Icon                string         `json:"icon,omitempty"`
	Component           string         `json:"component,omitempty"`
	Hidden              bool           `json:"hidden"`
	Sort                int            `json:"sort"`
	KeepAlive           bool           `json:"keep_alive"`
	ActiveMenu          string         `json:"active_menu,omitempty"`
	CloseTab            bool           `json:"close_tab"`
	IsBasic             bool           `json:"is_basic"`
	TransitionAnimation string         `json:"transition_animation,omitempty"`
	Children            []MenuTreeNode `json:"children,omitempty"`
}
