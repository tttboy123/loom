package production

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"

	_ "modernc.org/sqlite"
)

const prodJourney = "55555555-5555-4555-8555-555555555555"

func openProdStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/prod.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return journal.NewStore(db)
}

func prodTempDir(t testing.TB) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func mustProdService(t testing.TB, store *journal.Store, adminLock bool) (*Service, Paths) {
	t.Helper()
	appSupport := filepath.Join(prodTempDir(t), "Application Support", "Loom")
	launchAgents := filepath.Join(prodTempDir(t), "LaunchAgents")
	if err := os.MkdirAll(appSupport, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(launchAgents, 0o700); err != nil {
		t.Fatal(err)
	}
	paths := Paths{AppSupport: appSupport, LaunchAgents: launchAgents, DaemonPath: "/usr/local/bin/loomd"}
	service, err := NewService(store, paths, func() time.Time {
		return time.Date(2026, 8, 5, 15, 0, 0, 0, time.UTC)
	}, func(context.Context) (bool, error) { return adminLock, nil })
	if err != nil {
		t.Fatal(err)
	}
	return service, paths
}

func TestPreviewIsZeroWrite(t *testing.T) {
	store := openProdStore(t)
	service, paths := mustProdService(t, store, false)
	before := filesUnder(paths)
	result, err := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "default",
		JourneyID: prodJourney, OperationID: "op-preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Preview.Digest == "" || len(result.Preview.Files) != 2 {
		t.Fatalf("preview = %+v", result.Preview)
	}
	if !equalStrings(before, filesUnder(paths)) {
		t.Fatal("preview wrote files")
	}
}

func TestConfirmRequiresDigestAndAuth(t *testing.T) {
	store := openProdStore(t)
	service, _ := mustProdService(t, store, false)
	preview, err := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "default",
		JourneyID: prodJourney, OperationID: "op-preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	confirm := ActivationCommand{
		Operation: "activation_confirm", TargetMode: "default",
		PreviewDigest: preview.Digest, JourneyID: prodJourney, OperationID: "op-confirm",
	}
	if _, err := service.Command(context.Background(), confirm); !errors.Is(err, ErrAuthorizationRequired) {
		t.Fatalf("missing auth error = %v", err)
	}
	confirm.AuthorizedBy = "user-1"
	confirm.PreviewDigest = "bad"
	if _, err := service.Command(context.Background(), confirm); !errors.Is(err, ErrPreviewDigestMismatch) {
		t.Fatalf("digest error = %v", err)
	}
}

func TestActivationWritesJournalThenFilesAndRollsBackOnFailure(t *testing.T) {
	store := openProdStore(t)
	service, paths := mustProdService(t, store, false)
	preview, _ := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "default",
		JourneyID: prodJourney, OperationID: "op-preview",
	})
	confirm := ActivationCommand{
		Operation: "activation_confirm", TargetMode: "default",
		PreviewDigest: preview.Digest, AuthorizedBy: "user-1",
		JourneyID: prodJourney, OperationID: "op-confirm",
	}
	result, err := service.Command(context.Background(), confirm)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Activated {
		t.Fatalf("result = %+v", result)
	}
	if digestFile(paths.ConfigPath()) == "absent" || digestFile(paths.PlistPath()) == "absent" {
		t.Fatal("activation did not write config/plist")
	}
	events, _ := store.ReadAll(context.Background())
	if countProdEvents(events, EventActivationActivated) != 1 {
		t.Fatalf("activation fact count = %d", countProdEvents(events, EventActivationActivated))
	}
	// Failure rollback: make plist path a directory so install fails.
	_ = os.Remove(paths.PlistPath())
	if err := os.MkdirAll(paths.PlistPath(), 0o700); err != nil {
		t.Fatal(err)
	}
	preview2, _ := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "default",
		JourneyID: prodJourney, OperationID: "op-preview-2",
	})
	if _, err := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_confirm", TargetMode: "default",
		PreviewDigest: preview2.Digest, AuthorizedBy: "user-1",
		JourneyID: prodJourney, OperationID: "op-confirm-2",
	}); err == nil {
		t.Fatal("confirm with failing plist must error")
	}
	events, _ = store.ReadAll(context.Background())
	if countProdEvents(events, EventConfigRolledBack) == 0 {
		t.Fatal("missing rollback fact")
	}
	_ = os.Remove(paths.PlistPath())
}

func TestDeactivationRestoresBackup(t *testing.T) {
	store := openProdStore(t)
	service, paths := mustProdService(t, store, false)
	original := []byte(`{"schema_version":1,"activated":false,"original":true}`)
	if err := os.WriteFile(paths.ConfigPath(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	preview, _ := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "default",
		JourneyID: prodJourney, OperationID: "op-preview",
	})
	if _, err := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_confirm", TargetMode: "default",
		PreviewDigest: preview.Digest, AuthorizedBy: "user-1",
		JourneyID: prodJourney, OperationID: "op-confirm",
	}); err != nil {
		t.Fatal(err)
	}
	depreview, err := service.Command(context.Background(), ActivationCommand{
		Operation: "deactivation_preview", JourneyID: prodJourney, OperationID: "op-depreview",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Command(context.Background(), ActivationCommand{
		Operation: "deactivation_confirm", PreviewDigest: depreview.Digest,
		AuthorizedBy: "user-1", JourneyID: prodJourney, OperationID: "op-deconfirm",
	}); err != nil {
		t.Fatal(err)
	}
	if digestFile(paths.ConfigPath()) != sha256Hex(original) {
		t.Fatal("deactivation did not restore config backup")
	}
	if digestFile(paths.PlistPath()) != "absent" {
		t.Fatal("deactivation left plist")
	}
	events, _ := store.ReadAll(context.Background())
	if countProdEvents(events, EventActivationDeactivated) != 1 {
		t.Fatal("missing deactivation fact")
	}
}

func TestRecoveryDegradesOnConfigMismatch(t *testing.T) {
	store := openProdStore(t)
	service, paths := mustProdService(t, store, false)
	preview, _ := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "default",
		JourneyID: prodJourney, OperationID: "op-preview",
	})
	if _, err := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_confirm", TargetMode: "default",
		PreviewDigest: preview.Digest, AuthorizedBy: "user-1",
		JourneyID: prodJourney, OperationID: "op-confirm",
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Snapshot(context.Background())
	if err != nil || snapshot.Recovery.Degraded {
		t.Fatalf("fresh activation snapshot = %+v err=%v", snapshot, err)
	}
	if err := os.WriteFile(paths.ConfigPath(), []byte(`{"schema_version":1,"activated":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = service.Snapshot(context.Background())
	if err != nil || !snapshot.Recovery.Degraded {
		t.Fatalf("tampered snapshot = %+v err=%v", snapshot, err)
	}
}

func TestAdminLockBlocksBypassActivation(t *testing.T) {
	store := openProdStore(t)
	service, _ := mustProdService(t, store, true)
	preview, _ := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "bypass_permissions",
		JourneyID: prodJourney, OperationID: "op-preview",
	})
	if !preview.Preview.AdminLock {
		t.Fatal("preview must expose admin lock")
	}
	if _, err := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_confirm", TargetMode: "bypass_permissions",
		PreviewDigest: preview.Digest, AuthorizedBy: "user-1",
		JourneyID: prodJourney, OperationID: "op-confirm",
	}); !errors.Is(err, ErrAdminLockBlocksBypass) {
		t.Fatalf("admin lock error = %v", err)
	}
}

func TestActivationIsIdempotentAndPrivateModes(t *testing.T) {
	store := openProdStore(t)
	service, paths := mustProdService(t, store, false)
	preview, _ := service.Command(context.Background(), ActivationCommand{
		Operation: "activation_preview", TargetMode: "default",
		JourneyID: prodJourney, OperationID: "op-preview",
	})
	confirm := ActivationCommand{
		Operation: "activation_confirm", TargetMode: "default",
		PreviewDigest: preview.Digest, AuthorizedBy: "user-1",
		JourneyID: prodJourney, OperationID: "op-confirm",
	}
	if _, err := service.Command(context.Background(), confirm); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Command(context.Background(), confirm); err != nil {
		t.Fatalf("idempotent confirm error = %v", err)
	}
	events, _ := store.ReadAll(context.Background())
	if countProdEvents(events, EventActivationActivated) != 1 {
		t.Fatalf("activation fact count = %d", countProdEvents(events, EventActivationActivated))
	}
	for _, path := range []string{paths.ConfigPath(), paths.PlistPath()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode for %s = %o", path, info.Mode().Perm())
		}
	}
	appInfo, err := os.Stat(paths.AppSupport)
	if err != nil {
		t.Fatal(err)
	}
	if appInfo.Mode().Perm() != 0o700 {
		t.Fatalf("app support mode = %o", appInfo.Mode().Perm())
	}
}

func filesUnder(paths Paths) []string {
	var result []string
	for _, dir := range []string{paths.AppSupport, paths.LaunchAgents} {
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				result = append(result, path)
			}
			return nil
		})
	}
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func countProdEvents(events []journal.Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}
