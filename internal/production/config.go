package production

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	configFileName   = "daemon.json"
	configBackupName = "daemon.json.prev"
	plistBackupName  = "com.loom.local.daemon.plist.prev"
	privateDirMode   = 0o700
	privateFileMode  = 0o600
)

type configPayload struct {
	SchemaVersion int    `json:"schema_version"`
	Activated     bool   `json:"activated"`
	Mode          string `json:"mode"`
	ActivatedAt   string `json:"activated_at,omitempty"`
	DeactivatedAt string `json:"deactivated_at,omitempty"`
}

func desiredConfig(activated bool, mode string) []byte {
	payload := configPayload{
		SchemaVersion: 1, Activated: activated, Mode: mode,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}

func desiredPlist(daemonPath, appSupport string) []byte {
	statePath := filepath.Join(appSupport, "run", "loomd.db")
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.loom.local.daemon</string>
  <key>ProgramArguments</key><array>
    <string>` + daemonPath + `</string>
    <string>--state</string><string>` + statePath + `</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict>
</plist>
`)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func digestFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "absent"
	}
	return sha256Hex(data)
}

func ensurePrivateDir(path string) error {
	if path == "" {
		return ErrInvalidProductionInput
	}
	if err := os.MkdirAll(path, privateDirMode); err != nil {
		return err
	}
	return os.Chmod(path, privateDirMode)
}

// atomicWrite writes content to a temp file in the same directory, syncs it,
// and renames it over the target. It never follows symlinks.
func atomicWrite(path string, content []byte, mode os.FileMode) error {
	if path == "" {
		return ErrInvalidProductionInput
	}
	dir := filepath.Dir(path)
	if err := ensurePrivateDir(dir); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("production target is a symlink")
	}
	temp, err := os.CreateTemp(dir, ".loom-prod-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		if tempPath != "" {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	tempPath = ""
	return nil
}

func copyFileAtomic(source, target string) error {
	if source == "" || target == "" {
		return ErrInvalidProductionInput
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return atomicWrite(target, data, privateFileMode)
}

func removeFileIfExists(path string) error {
	if path == "" {
		return ErrInvalidProductionInput
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != "" && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

var _ = io.EOF
