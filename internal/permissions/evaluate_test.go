package permissions

import (
	"testing"
)

func TestRed04_ReadOnlyWhitelistZeroPrompt(t *testing.T) {
	effective := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeDefault},
		Mode:    ModeDefault,
	}
	readOnly := []ProposedCall{
		{Tool: ToolRead, Path: "src/main.go"},
		{Tool: ToolGrep, Path: "src/**"},
		{Tool: ToolWebSearch, Path: "query"},
		{Tool: ToolBash, Command: "ls -la"},
		{Tool: ToolBash, Command: "cat go.mod"},
		{Tool: ToolBash, Command: "git status"},
		{Tool: ToolBash, Command: "git log --oneline"},
		{Tool: ToolBash, Command: "git diff"},
		{Tool: ToolBash, Command: "rg error"},
		{Tool: ToolBash, Command: "pwd"},
	}
	for _, call := range readOnly {
		if got := verdictFor(t, effective, call); got != VerdictAllow {
			t.Fatalf("read-only call %+v verdict = %s, want allow", call, got)
		}
	}
	notReadOnly := []ProposedCall{
		{Tool: ToolBash, Command: "tee /tmp/out.txt"},
		{Tool: ToolBash, Command: "go test ./..."},
		{Tool: ToolBash, Command: "go build ./..."},
		{Tool: ToolBash, Command: "curl https://example.com"},
	}
	for _, call := range notReadOnly {
		if got := verdictFor(t, effective, call); got != VerdictAsk {
			t.Fatalf("non-read-only call %+v verdict = %s, want ask in default", call, got)
		}
	}
}

func TestRed04_DefaultTemplateCoversDevCommands(t *testing.T) {
	template, err := DefaultProjectProfileTemplate()
	if err != nil {
		t.Fatalf("DefaultProjectProfileTemplate() error = %v", err)
	}
	if template.Mode != ModeDefault {
		t.Fatalf("template mode = %s, want default", template.Mode)
	}
	effective := EffectiveProfile{
		Profile:     template,
		Mode:        template.Mode,
		MergedRules: template.Rules,
	}
	for _, call := range []ProposedCall{
		{Tool: ToolBash, Command: "go test ./..."},
		{Tool: ToolBash, Command: "go build ./..."},
		{Tool: ToolBash, Command: "gofmt -l ."},
		{Tool: ToolBash, Command: "go vet ./..."},
		{Tool: ToolBash, Command: "git status"},
		{Tool: ToolBash, Command: "git diff"},
	} {
		if got := verdictFor(t, effective, call); got != VerdictAllow {
			t.Fatalf("template should allow %q, got %s", call.Command, got)
		}
	}
	for _, call := range []ProposedCall{
		{Tool: ToolBash, Command: "rm -rf vendor"},
		{Tool: ToolBash, Command: "git push origin main"},
		{Tool: ToolBash, Command: "curl https://example.com"},
	} {
		if got := verdictFor(t, effective, call); got == VerdictAllow {
			t.Fatalf("template must NOT allow dangerous/network %q, got allow", call.Command)
		}
	}
}

func TestRed06_ChainedCommandDenyWholeAllowWhole(t *testing.T) {
	effective := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeDefault},
		Mode:    ModeDefault,
		MergedRules: []Rule{
			{RuleID: "r-allow-git", Scope: ScopeProject, ScopeID: permTestProject, Action: ActionAllow, Tool: ToolBash, Pattern: "git *"},
			{RuleID: "r-deny-rm", Scope: ScopeRoot, ScopeID: "global", Action: ActionDeny, Tool: ToolBash, Pattern: "rm -rf *"},
		},
	}
	call := ProposedCall{Tool: ToolBash, Command: "git status && rm -rf /tmp/x"}
	if got := verdictFor(t, effective, call); got != VerdictDeny {
		t.Fatalf("chained call verdict = %s, want deny (rm segment denied)", got)
	}

	whole := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeDefault},
		Mode:    ModeDefault,
		MergedRules: []Rule{
			{RuleID: "r-allow-whole", Scope: ScopeProject, ScopeID: permTestProject, Action: ActionAllow, Tool: ToolBash, Pattern: "git status && ls *"},
		},
	}
	if got := verdictFor(t, whole, ProposedCall{Tool: ToolBash, Command: "git status && ls -la"}); got != VerdictAllow {
		t.Fatalf("whole-string allow verdict = %s, want allow", got)
	}
	if got := verdictFor(t, whole, ProposedCall{Tool: ToolBash, Command: "git status && cat go.mod"}); got != VerdictAsk {
		t.Fatalf("allow must match whole string only, verdict = %s, want ask", got)
	}
}

func TestRed18_AllowRuleCannotPassDangerousChain(t *testing.T) {
	template, err := DefaultProjectProfileTemplate()
	if err != nil {
		t.Fatal(err)
	}
	effective := EffectiveProfile{
		Profile:     template,
		Mode:        template.Mode,
		MergedRules: template.Rules,
	}
	for _, call := range []ProposedCall{
		{Tool: ToolBash, Command: "go test ./... && rm -rf vendor"},
		{Tool: ToolBash, Command: "git status && rm -rf /tmp/x"},
		{Tool: ToolBash, Command: "go build ./... ; chmod +x run.sh"},
	} {
		if got := verdictFor(t, effective, call); got != VerdictAsk {
			t.Fatalf("dangerous chain %q verdict = %s, want ask (allow rule must not pass it)", call.Command, got)
		}
	}
	// An explicit whole-string allow rule must still not pass a dangerous chain.
	whole := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeDefault},
		Mode:    ModeDefault,
		MergedRules: []Rule{
			{RuleID: "r-allow-git", Scope: ScopeProject, ScopeID: permTestProject,
				Action: ActionAllow, Tool: ToolBash, Pattern: "git *"},
		},
	}
	if got := verdictFor(t, whole, ProposedCall{Tool: ToolBash, Command: "git status && rm -rf /tmp/x"}); got != VerdictAsk {
		t.Fatalf("allow rule must not pass chained rm, verdict = %s", got)
	}
	// bypass permissions still re-asks on dangerous segments.
	bypass := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeBypassPermissions},
		Mode:    ModeBypassPermissions,
	}
	if got := verdictFor(t, bypass, ProposedCall{Tool: ToolBash, Command: "git status && rm -rf /tmp/x"}); got != VerdictAsk {
		t.Fatalf("bypass must still ask on dangerous chain, verdict = %s", got)
	}
	// A clean template command remains allowed (no UX regression).
	if got := verdictFor(t, effective, ProposedCall{Tool: ToolBash, Command: "go test ./..."}); got != VerdictAllow {
		t.Fatalf("clean template command verdict = %s, want allow", got)
	}
}

func TestRed09_UnknownToolAndParseFailureError(t *testing.T) {
	effective := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeDefault},
		Mode:    ModeDefault,
	}
	if _, _, err := Evaluate(effective, ProposedCall{Tool: "Frobnicate", Command: "x"}); err == nil {
		t.Fatal("unknown tool must error")
	}
	if _, _, err := Evaluate(effective, ProposedCall{Tool: ToolBash, Command: ""}); err == nil {
		t.Fatal("empty bash command must error")
	}
	if _, _, err := Evaluate(effective, ProposedCall{Tool: ToolRead, Path: ""}); err == nil {
		t.Fatal("empty read path must error")
	}
	if _, err := CompileProfile(ProfileInput{
		ProfileID: "p", Mode: "bogus-mode",
	}); err == nil {
		t.Fatal("invalid mode must error")
	}
	if _, err := CompileProfile(ProfileInput{
		ProfileID: "p", Mode: ModeDefault,
		Rules: []Rule{{RuleID: "", Scope: ScopeProject, ScopeID: permTestProject, Action: ActionAllow, Tool: ToolBash, Pattern: "go test *"}},
	}); err == nil {
		t.Fatal("empty rule id must error")
	}
}

func TestModeSemantics(t *testing.T) {
	base := EffectiveProfile{Profile: PermissionProfile{ProfileID: "p", Mode: ModeDefault}, Mode: ModeDefault}
	readCall := ProposedCall{Tool: ToolRead, Path: "src/main.go"}
	editCall := ProposedCall{Tool: ToolEdit, Path: "src/main.go"}
	owned := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeAcceptEdits, OwnedPaths: []string{"src/**"}},
		Mode:    ModeAcceptEdits,
	}
	if got := verdictFor(t, base, readCall); got != VerdictAllow {
		t.Fatalf("default read verdict = %s, want allow", got)
	}
	if got := verdictFor(t, owned, editCall); got != VerdictAllow {
		t.Fatalf("accept_edits owned-path edit verdict = %s, want allow", got)
	}
	if got := verdictFor(t, base, editCall); got != VerdictAsk {
		t.Fatalf("default edit verdict = %s, want ask", got)
	}
	dontAsk := EffectiveProfile{
		Profile: PermissionProfile{ProfileID: "p", Mode: ModeDontAsk},
		Mode:    ModeDontAsk,
	}
	if got := verdictFor(t, dontAsk, editCall); got != VerdictDeny {
		t.Fatalf("dont_ask edit verdict = %s, want deny", got)
	}
}

func TestPathGlobSemantics(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"src/*", "src/main.go", true},
		{"src/*", "src/nested/main.go", false},
		{"src/**", "src/nested/main.go", true},
		{"**/.env", ".env", true},
		{"**/.env", "config/.env", true},
		{"src/**", "internal/x.go", false},
	}
	for _, tc := range cases {
		if got := matchPathGlob(tc.pattern, tc.path); got != tc.want {
			t.Fatalf("matchPathGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestDangerousSegments(t *testing.T) {
	command := "git status && rm -rf vendor; chmod +x run.sh | cat"
	segments := DangerousSegments(command)
	if len(segments) != 2 {
		t.Fatalf("DangerousSegments() = %v, want 2", segments)
	}
	if segments[0] != "rm -rf vendor" || segments[1] != "chmod +x run.sh" {
		t.Fatalf("unexpected dangerous segments: %v", segments)
	}
}

func TestIsReadOnlyWordBoundary(t *testing.T) {
	if !IsReadOnlyCommand("ls -la") {
		t.Fatal("ls must be read-only")
	}
	if IsReadOnlyCommand("lsof /tmp") {
		t.Fatal("lsof must not match ls (word boundary)")
	}
	if !IsReadOnlyCommand("git status") {
		t.Fatal("git status must be read-only")
	}
	if IsReadOnlyCommand("git commit -m x") {
		t.Fatal("git commit must not be read-only")
	}
	if IsReadOnlyCommand("git config --global user.email a@b.c") {
		t.Fatal("git config must not be read-only")
	}
	if IsReadOnlyCommand("cat > /tmp/out.txt") {
		t.Fatal("cat with redirection must not be read-only")
	}
}
