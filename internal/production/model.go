// Package production implements C-W1 Production Landing: an explicit,
// auditable activation flow for the resident Loom daemon. Activation preview
// is zero-write; confirm writes the Journal fact first, then atomically
// installs Application Support configuration and a user-level launchd plist;
// deactivation restores backups. Recovery is fail-closed to read-only when
// the Journal says activated but the disk does not match.
package production

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidProductionInput = errors.New("invalid production input")
	ErrPreviewDigestMismatch  = errors.New("production preview digest mismatch")
	ErrAuthorizationRequired  = errors.New("production activation requires authorized_by")
	ErrAdminLockBlocksBypass  = errors.New("admin lock blocks bypass permissions activation")
	ErrDegraded               = errors.New("production state degraded; writes are blocked")
)

const (
	EventActivationActivated   = "ProductionActivationActivated"
	EventActivationDeactivated = "ProductionActivationDeactivated"
	EventConfigWritten         = "ProductionConfigWritten"
	EventConfigRolledBack      = "ProductionConfigRolledBack"
	streamActivation           = "production-activation"
	streamConfigPrefix         = "production-config/"
)

type Paths struct {
	AppSupport   string
	LaunchAgents string
	DaemonPath   string
}

func (p Paths) ConfigPath() string {
	if p.AppSupport == "" {
		return ""
	}
	return p.AppSupport + "/daemon.json"
}

func (p Paths) PlistPath() string {
	if p.LaunchAgents == "" {
		return ""
	}
	return p.LaunchAgents + "/com.loom.local.daemon.plist"
}

type FileChange struct {
	Path          string `json:"path"`
	Action        string `json:"action"` // write, remove, restore
	CurrentDigest string `json:"current_digest"`
	DesiredDigest string `json:"desired_digest"`
	Mode          string `json:"mode"`
}

type LaunchdChange struct {
	Label     string `json:"label"`
	PlistPath string `json:"plist_path"`
	Action    string `json:"action"` // install, remove
}

type ActivationPreview struct {
	Operation    string          `json:"operation"` // activation, deactivation
	Scope        string          `json:"scope"`
	TargetMode   string          `json:"target_mode"`
	Digest       string          `json:"digest"`
	Files        []FileChange    `json:"files"`
	LaunchdItems []LaunchdChange `json:"launchd_items"`
	AdminLock    bool            `json:"admin_lock"`
}

type RecoveryStatus struct {
	Degraded             bool   `json:"degraded"`
	Reason               string `json:"reason,omitempty"`
	LastActivated        bool   `json:"last_activated"`
	ConfigDigestMismatch bool   `json:"config_digest_mismatch"`
}

type ProductionSnapshot struct {
	ViewVersion string         `json:"view_version"`
	Activated   bool           `json:"activated"`
	ActivatedAt string         `json:"activated_at,omitempty"`
	TargetMode  string         `json:"target_mode,omitempty"`
	Recovery    RecoveryStatus `json:"recovery"`
	LastPreview string         `json:"last_preview,omitempty"`
}

type ActivationCommand struct {
	Operation     string
	TargetMode    string
	PreviewDigest string
	AuthorizedBy  string
	JourneyID     string
	OperationID   string
}

type CommandResult struct {
	Operation string            `json:"operation"`
	Preview   ActivationPreview `json:"preview,omitempty"`
	Digest    string            `json:"digest,omitempty"`
	Activated bool              `json:"activated,omitempty"`
	Recovery  RecoveryStatus    `json:"recovery,omitempty"`
	EventIDs  []string          `json:"event_ids,omitempty"`
	Note      string            `json:"note,omitempty"`
}

// AdminLockReader is injected by the daemon; it reads the permissions
// projection without writing.
type AdminLockReader func(context.Context) (bool, error)

type Now func() time.Time
