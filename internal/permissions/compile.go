package permissions

import "strings"

// CompileProfile validates and canonicalizes a ProfileInput into an immutable
// PermissionProfile with generation 1 and a canonical digest.
func CompileProfile(input ProfileInput) (PermissionProfile, error) {
	if !validID(input.ProfileID) {
		return PermissionProfile{}, ErrInvalidInput
	}
	if !ValidMode(string(input.Mode)) {
		return PermissionProfile{}, ErrInvalidMode
	}
	seen := make(map[string]bool, len(input.Rules))
	rules := cloneRules(input.Rules)
	for _, rule := range rules {
		if err := validateRule(rule); err != nil {
			return PermissionProfile{}, err
		}
		if seen[rule.RuleID] {
			return PermissionProfile{}, ErrInvalidRule
		}
		seen[rule.RuleID] = true
	}
	for _, path := range input.OwnedPaths {
		if strings.TrimSpace(path) == "" {
			return PermissionProfile{}, ErrInvalidInput
		}
	}
	digest, err := digestProfile(input.ProfileID, input.Mode, rules, input.OwnedPaths)
	if err != nil {
		return PermissionProfile{}, err
	}
	return PermissionProfile{
		ProfileID:  input.ProfileID,
		Generation: 1,
		Digest:     digest,
		Mode:       input.Mode,
		Rules:      rules,
		OwnedPaths: clonePaths(input.OwnedPaths),
	}, nil
}

// DefaultProjectProfileTemplate returns the frozen default project profile:
// standard development commands are explicit allow rules so routine test and
// build journeys are zero-prompt, while network, MCP, dangerous and
// delete/overwrite patterns remain ask/deny in default mode.
func DefaultProjectProfileTemplate() (PermissionProfile, error) {
	rules := []Rule{
		{RuleID: "template-go-test", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "go test *"},
		{RuleID: "template-go-build", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "go build *"},
		{RuleID: "template-gofmt", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "gofmt *"},
		{RuleID: "template-go-vet", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "go vet *"},
		{RuleID: "template-git-status", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "git status"},
		{RuleID: "template-git-log", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "git log *"},
		{RuleID: "template-git-diff", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "git diff"},
		{RuleID: "template-git-show", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "git show *"},
		{RuleID: "template-rg", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "rg *"},
		{RuleID: "template-grep", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "grep *"},
		{RuleID: "template-ls", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "ls *"},
		{RuleID: "template-cat", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolBash, Pattern: "cat *"},
		{RuleID: "template-read", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolRead, Pattern: "**"},
		{RuleID: "template-grep-tool", Scope: ScopeProject, ScopeID: "default", Action: ActionAllow, Tool: ToolGrep, Pattern: "**"},
	}
	return CompileProfile(ProfileInput{
		ProfileID: "default-project-template",
		Mode:      ModeDefault,
		Rules:     rules,
	})
}
