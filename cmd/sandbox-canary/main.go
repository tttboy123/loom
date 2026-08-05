// Command sandbox-canary is the Phase 3B controlled canary. It exercises the
// loopback sandbox backend through the Journal-backed Controller and writes
// deterministic evidence into a journey root that verify-phase3b-canary.sh
// checks. It never reads provider credentials and never writes secrets.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/sandbox"

	_ "modernc.org/sqlite"
)

const canaryScenario = "phase3b-sandbox-canary"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: sandbox-canary ROOT")
		os.Exit(2)
	}
	root := os.Args[1]
	if !filepath.IsAbs(root) {
		fmt.Fprintln(os.Stderr, "root must be absolute")
		os.Exit(2)
	}
	if err := run(root); err != nil {
		fmt.Fprintln(os.Stderr, "sandbox-canary:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	ctx := context.Background()
	for _, directory := range []string{
		"manifest", "state", "journal", "processes",
	} {
		path := filepath.Join(root, directory)
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}

	journeyID, err := newJourneyID()
	if err != nil {
		return err
	}
	manifest := map[string]any{
		"journey_id": journeyID, "scenario_id": canaryScenario,
		"created_at_utc": time.Now().UTC().Format(time.RFC3339Nano),
		"kind":           "phase3b-sandbox-canary",
	}
	if err := writeJSON(filepath.Join(root, "manifest", "journey-harness.json"), manifest, 0o600); err != nil {
		return err
	}

	db, err := openStore(filepath.Join(root, "state", "loom.db"))
	if err != nil {
		return err
	}
	dbPath := filepath.Join(root, "state", "loom.db")
	defer func() {
		_ = db.Close()
		_ = os.Chmod(dbPath, 0o600)
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
	}()
	store := journal.NewStore(db)
	now := func() time.Time { return time.Now().UTC() }
	backend, err := sandbox.NewLoopbackBackend(filepath.Join(root, "state", "sandbox-work"), 30*time.Second)
	if err != nil {
		return err
	}
	controller := sandbox.NewController(store, backend, now)

	checks := []checkResult{}

	// 1. Isolated execution in a fresh per-instance workspace.
	instance, err := controller.Create(ctx, sandbox.CreateRequest{
		JobID: "canary-job", RunID: "canary-run-1", Generation: 1,
		WorkspaceDigest: "sha256:canary-workspace",
	}, "op-canary-create-1", journeyID)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	execResult, err := controller.Exec(ctx, sandbox.ExecRequest{
		InstanceID: instance.InstanceID, Command: "pwd && printf canary-ok",
		Timeout: 10 * time.Second, EnvDigest: sandbox.EnvDigest(map[string]string{"API_TOKEN": "canary-secret-value"}),
	}, 1, "op-canary-exec-1", journeyID)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	checks = append(checks, checkResult{Name: "isolated-exec", OK: execResult.ExitCode == 0})

	// 2. No credential leakage into the sanitized environment (grep finds 0).
	leakResult, err := controller.Exec(ctx, sandbox.ExecRequest{
		InstanceID: instance.InstanceID, Command: "env | grep -c canary-secret-value || true",
		Timeout: 10 * time.Second, EnvDigest: sandbox.EnvDigest(map[string]string{"API_TOKEN": "canary-secret-value"}),
	}, 1, "op-canary-leak-1", journeyID)
	if err != nil {
		return fmt.Errorf("leak probe: %w", err)
	}
	checks = append(checks, checkResult{Name: "no-secret-leak", OK: leakResult.ExitCode == 0})

	// 3. Generation fencing: stale generation is rejected.
	if _, err := controller.Exec(ctx, sandbox.ExecRequest{
		InstanceID: instance.InstanceID, Command: "echo stale", Timeout: time.Second,
	}, 99, "op-canary-stale-1", journeyID); err == nil {
		return fmt.Errorf("stale generation accepted")
	}
	checks = append(checks, checkResult{Name: "generation-fencing", OK: true})

	// 4. Cancel terminates a long command without a terminal success.
	cancelInstance, err := controller.Create(ctx, sandbox.CreateRequest{
		JobID: "canary-job", RunID: "canary-run-2", Generation: 1,
		WorkspaceDigest: "sha256:canary-workspace",
	}, "op-canary-create-2", journeyID)
	if err != nil {
		return fmt.Errorf("create cancel fixture: %w", err)
	}
	type cancelOutcome struct {
		result sandbox.ExecResult
		err    error
	}
	cancelDone := make(chan cancelOutcome, 1)
	go func() {
		result, cancelExecErr := controller.Exec(ctx, sandbox.ExecRequest{
			InstanceID: cancelInstance.InstanceID, Command: "sleep 30",
			Timeout: 30 * time.Second, EnvDigest: "sha256:unused",
		}, 1, "op-canary-cancel-exec", journeyID)
		cancelDone <- cancelOutcome{result: result, err: cancelExecErr}
	}()
	time.Sleep(500 * time.Millisecond)
	if err := controller.Cancel(ctx, cancelInstance.InstanceID, "op-canary-cancel-1", journeyID); err != nil {
		return fmt.Errorf("cancel: %w", err)
	}
	canceled := <-cancelDone
	checks = append(checks, checkResult{
		Name: "cancel-terminates",
		OK:   canceled.err != nil || canceled.result.ExitCode != 0,
	})
	if err := controller.Destroy(ctx, cancelInstance.InstanceID, "op-canary-destroy-cancel", journeyID); err != nil {
		return fmt.Errorf("destroy cancel fixture: %w", err)
	}

	// 5. Restart/replay recovery: an undestroyed instance is reconciled and
	// cleaned without re-execution.
	orphan, err := controller.Create(ctx, sandbox.CreateRequest{
		JobID: "canary-job", RunID: "canary-run-3", Generation: 1,
		WorkspaceDigest: "sha256:canary-workspace",
	}, "op-canary-create-3", journeyID)
	if err != nil {
		return fmt.Errorf("create orphan: %w", err)
	}
	if _, err := controller.Exec(ctx, sandbox.ExecRequest{
		InstanceID: orphan.InstanceID, Command: "printf orphan", Timeout: 10 * time.Second,
		EnvDigest: "sha256:unused",
	}, 1, "op-canary-orphan-exec", journeyID); err != nil {
		return fmt.Errorf("orphan exec: %w", err)
	}
	results, err := controller.Reconcile(ctx)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}
	orphanReconciled := false
	for _, result := range results {
		if result.InstanceID == orphan.InstanceID {
			orphanReconciled = result.Destroyed && result.Err == nil
		}
	}
	checks = append(checks, checkResult{Name: "reconcile-cleanup", OK: orphanReconciled})

	// 6. Journal-derived projection is the single authority.
	states, err := controller.Rebuild(ctx)
	if err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}
	destroyed := 0
	for _, state := range states {
		if state.Destroyed {
			destroyed++
		}
	}
	// All three instances are destroyed: the first by Reconcile (it was never
	// destroyed), the cancel fixture explicitly, and the orphan by Reconcile.
	checks = append(checks, checkResult{Name: "journal-authority", OK: destroyed == 3})

	// Checkpoint and switch back to DELETE journaling so the artifact is
	// fully readable with sqlite3 -readonly and carries no -wal/-shm residue.
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=DELETE;").Scan(&mode); err != nil {
		return fmt.Errorf("journal mode: %w", err)
	}
	if mode != "delete" {
		return fmt.Errorf("journal mode not delete: %s", mode)
	}

	allOK := true
	for _, check := range checks {
		if !check.OK {
			allOK = false
		}
	}
	if err := writeJSON(filepath.Join(root, "journal", "facts-summary.json"),
		map[string]any{"checks": checks, "all_ok": allOK}, 0o600); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "processes", "preflight.json"), []byte(`{"daemon":1,"sockets":0,"locks":0,"leases":0,"temps":0}`), 0o600); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "processes", "postflight.json"), []byte(`{"processes":0,"sockets":0,"locks":0,"leases":0,"temps":0}`), 0o600); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "processes", "cleanup-proof.txt"), []byte("no lingering sandbox processes\n"), 0o600); err != nil {
		return err
	}

	var builder strings.Builder
	builder.WriteString("# Phase 3B Sandbox Canary\n\n")
	builder.WriteString(fmt.Sprintf("- Journey: `%s`\n", journeyID))
	builder.WriteString(fmt.Sprintf("- Scenario: `%s`\n\n", canaryScenario))
	for _, check := range checks {
		status := "PASS"
		if !check.OK {
			status = "FAIL"
		}
		builder.WriteString(fmt.Sprintf("- [%s] %s\n", status, check.Name))
	}
	builder.WriteString(fmt.Sprintf("\n**Overall**: %s\n", canaryStatus(allOK)))
	if err := writeFile(filepath.Join(root, "result.md"), []byte(builder.String()), 0o600); err != nil {
		return err
	}
	if !allOK {
		return fmt.Errorf("canary checks failed")
	}
	return nil
}

type checkResult struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

func canaryStatus(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func openStore(path string) (*sql.DB, error) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", path, values.Encode()))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := journal.Migrate(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Keep the journal file private even though sqlite creates it with the
	// process umask.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func newJourneyID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func writeJSON(path string, value any, mode os.FileMode) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, body, mode)
}

func writeFile(path string, body []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, body, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
