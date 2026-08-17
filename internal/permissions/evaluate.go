package permissions

import (
	"sort"
	"strings"
)

// IsReadOnlyTool reports whether a tool kind is on the read-only whitelist.
func IsReadOnlyTool(kind ToolKind) bool {
	// WebSearch and WebFetch are both read-only network operations: the
	// SSRF-safe transport performs a bounded GET and never mutates local or
	// remote state. Treating WebFetch as read-only keeps it usable inside a
	// Mission (no interactive approval channel) exactly like WebSearch.
	return kind == ToolRead || kind == ToolGrep ||
		kind == ToolWebSearch || kind == ToolWebFetch
}

var readOnlyCommands = map[string]bool{
	"ls": true, "cat": true, "pwd": true, "head": true,
	"tail": true, "wc": true, "grep": true, "rg": true,
}

var readOnlyGitSubcommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "rev-parse": true,
}

// IsReadOnlyCommand recognizes a fixed read-only shell command set with
// word-boundary matching. Chains, pipes, redirection, substitution and
// backgrounding are never treated as read-only.
func IsReadOnlyCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}
	if strings.ContainsAny(trimmed, "&|;<>`$()") {
		return false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "git" {
		if len(fields) < 2 {
			return false
		}
		return readOnlyGitSubcommands[fields[1]]
	}
	return readOnlyCommands[fields[0]]
}

var dangerousCommands = map[string]bool{
	"rm": true, "chmod": true, "chown": true, "chattr": true,
	"pkill": true, "kill": true, "killall": true, "dd": true,
	"mkfs": true, "shutdown": true, "reboot": true,
}

// DangerousSegments returns the segments of a command that match the frozen
// dangerous command table (rm, chmod, chown, chattr, pkill, kill, killall, dd,
// mkfs, shutdown, reboot, git push, git reset --hard, git clean -f).
func DangerousSegments(command string) []string {
	var matched []string
	for _, segment := range splitCommandSegments(command) {
		if isDangerousSegment(segment) {
			matched = append(matched, strings.TrimSpace(segment))
		}
	}
	return matched
}

func isDangerousSegment(segment string) bool {
	fields := stripEnvPrefixes(strings.Fields(segment))
	if len(fields) == 0 {
		return false
	}
	cmd := fields[0]
	if dangerousCommands[cmd] {
		return true
	}
	if cmd == "git" && len(fields) >= 2 {
		switch fields[1] {
		case "push":
			return true
		case "reset":
			return len(fields) >= 3 && fields[2] == "--hard"
		case "clean":
			return len(fields) >= 3 && fields[2] == "-f"
		}
	}
	return false
}

func stripEnvPrefixes(fields []string) []string {
	start := 0
	for start < len(fields) {
		field := fields[start]
		if strings.Contains(field, "=") && !strings.Contains(field, "/") && !strings.HasPrefix(field, "-") {
			start++
			continue
		}
		break
	}
	return fields[start:]
}

func splitCommandSegments(command string) []string {
	replacer := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
	return strings.Split(replacer.Replace(command), "\n")
}

// Evaluate deterministically decides a ProposedCall against the effective
// profile. It is a pure function: any internal failure returns an error and
// the caller must treat it as deny (fail-closed).
func Evaluate(effective EffectiveProfile, call ProposedCall) (Verdict, Denial, error) {
	if !ValidToolKind(string(call.Tool)) {
		return "", Denial{}, ErrUnknownTool
	}
	switch call.Tool {
	case ToolBash:
		if strings.TrimSpace(call.Command) == "" {
			return "", Denial{}, ErrEmptyCommand
		}
	case ToolRead, ToolEdit, ToolGrep:
		if strings.TrimSpace(call.Path) == "" {
			return "", Denial{}, ErrEmptyPath
		}
	}
	if !ValidMode(string(effective.Mode)) {
		return "", Denial{}, ErrInvalidMode
	}
	if effective.AdminLock && effective.Mode == ModeBypassPermissions {
		return VerdictDeny, Denial{
			Reason:            "admin lock blocks bypass permissions mode",
			AuthorizationPath: "disable admin lock via a human-authorized writer",
		}, nil
	}

	dangerous := DangerousSegments(call.Command)

	// 1. Deny rules always win.
	denyIDs := matchedRuleIDs(effective.MergedRules, call, ActionDeny)
	if len(denyIDs) > 0 {
		return VerdictDeny, Denial{
			Reason:            "denied by permission rule",
			AuthorizationPath: "request a grant or an allow rule from the project owner",
			RuleIDs:           denyIDs,
		}, nil
	}

	// 2. Dangerous segments re-ask in every mode except dont_ask, before any
	//    allow rule or grant can pass the command. An allow rule matching the
	//    whole string must never open a chained dangerous segment (contract
	//    §3.4; RED #18).
	if len(dangerous) > 0 {
		if effective.Mode == ModeDontAsk {
			return VerdictDeny, Denial{
				Reason:            "dangerous command not allowed in dont_ask mode",
				AuthorizationPath: "switch to a less restrictive mode via a human-authorized activation",
			}, nil
		}
		return VerdictAsk, Denial{
			Reason:            "dangerous command requires approval",
			AuthorizationPath: "resolve the approval in the Attention Inbox",
		}, nil
	}

	// 3. Rules: deny > (narrowest matching scope; ask > allow within it).
	if ruleIDs, ok := askOverAllowAtNarrowestScope(effective.MergedRules, call); ok {
		return VerdictAsk, Denial{
			Reason:            "action requires approval",
			AuthorizationPath: "resolve the approval in the Attention Inbox",
			RuleIDs:           ruleIDs,
		}, nil
	}
	if ruleIDs := matchedRuleIDs(effective.MergedRules, call, ActionAllow); len(ruleIDs) > 0 {
		return VerdictAllow, Denial{}, nil
	}

	// 4. Active grants (dangerous patterns are never grantable; dangerous
	//    commands therefore never satisfy a grant).
	for _, grant := range effective.Grants {
		if grant.RevokedAt != "" || grant.Tool != call.Tool {
			continue
		}
		if call.Tool == ToolBash {
			if bashPatternMatches(grant.Pattern, call.Command) {
				return VerdictAllow, Denial{}, nil
			}
		} else if matchPathGlob(grant.Pattern, call.Path) {
			return VerdictAllow, Denial{}, nil
		}
	}

	// 5. Read-only whitelist.
	if IsReadOnlyTool(call.Tool) || (call.Tool == ToolBash && IsReadOnlyCommand(call.Command)) {
		return VerdictAllow, Denial{}, nil
	}

	// 6. Mode prompt policy.
	switch effective.Mode {
	case ModeDontAsk:
		return VerdictDeny, Denial{
			Reason:            "not in the dont_ask allowlist",
			AuthorizationPath: "add an explicit allow rule or grant",
		}, nil
	case ModeAcceptEdits:
		if call.Tool == ToolEdit && pathUnderOwned(effective.Profile.OwnedPaths, call.Path) {
			return VerdictAllow, Denial{}, nil
		}
		return VerdictAsk, Denial{
			Reason:            "edit outside owned paths requires approval",
			AuthorizationPath: "narrow the owned paths or add an allow rule",
		}, nil
	case ModeAuto:
		if call.Tool == ToolEdit && pathUnderOwned(effective.Profile.OwnedPaths, call.Path) {
			return VerdictAllow, Denial{}, nil
		}
		if call.Tool == ToolBash {
			return VerdictAllow, Denial{}, nil
		}
		if call.Tool == ToolMCPTool || call.Tool == ToolWebFetch {
			return VerdictAsk, Denial{
				Reason:            "network/MCP tool requires approval in auto mode",
				AuthorizationPath: "add an explicit allow rule",
			}, nil
		}
		return VerdictAllow, Denial{}, nil
	case ModeBypassPermissions:
		return VerdictAllow, Denial{}, nil
	default:
		return VerdictAsk, Denial{
			Reason:            "action requires approval",
			AuthorizationPath: "resolve the approval in the Attention Inbox",
		}, nil
	}
}

func matchedRuleIDs(rules []Rule, call ProposedCall, action RuleAction) []string {
	var ids []string
	for _, rule := range rules {
		if rule.Action != action || rule.Tool != call.Tool {
			continue
		}
		if ruleMatches(rule, call) {
			ids = append(ids, rule.RuleID)
		}
	}
	sort.Strings(ids)
	return ids
}

// askOverAllowAtNarrowestScope implements the intentional deviation from
// grok-build's severity-only merge: among matched non-deny rules, the narrowest
// scope wins, and ask beats allow within that scope.
func askOverAllowAtNarrowestScope(rules []Rule, call ProposedCall) ([]string, bool) {
	scopeOrder := map[ScopeKind]int{
		ScopeJob: 0, ScopeProject: 1, ScopePersonal: 2, ScopeRoot: 3,
	}
	matched := make(map[ScopeKind][]Rule)
	for _, rule := range rules {
		if (rule.Action != ActionAsk && rule.Action != ActionAllow) || rule.Tool != call.Tool {
			continue
		}
		if ruleMatches(rule, call) {
			matched[rule.Scope] = append(matched[rule.Scope], rule)
		}
	}
	if len(matched) == 0 {
		return nil, false
	}
	var narrowest ScopeKind
	best := 1 << 30
	for scope := range matched {
		order := scopeOrder[scope]
		if order < best {
			best = order
			narrowest = scope
		}
	}
	var ids []string
	var hasAsk bool
	for _, rule := range matched[narrowest] {
		ids = append(ids, rule.RuleID)
		if rule.Action == ActionAsk {
			hasAsk = true
		}
	}
	if hasAsk {
		sort.Strings(ids)
		return ids, true
	}
	// ask at this scope wins over allow at the same scope; a narrowest scope
	// with only allow rules falls through to the allow rule check.
	sort.Strings(ids)
	return nil, false
}

func ruleMatches(rule Rule, call ProposedCall) bool {
	if rule.Tool != call.Tool {
		return false
	}
	if rule.Tool == ToolBash {
		if rule.Action == ActionAllow {
			return bashPatternMatches(rule.Pattern, call.Command)
		}
		return bashSegmentMatches(rule.Pattern, call.Command)
	}
	return matchPathGlob(rule.Pattern, call.Path)
}

func bashPatternMatches(pattern, command string) bool {
	if pattern == "" {
		return false
	}
	// Allow rules match the whole command only (contract §3.2, RED #6/#20):
	// pattern and command must contain the same number of segments, and each
	// pattern segment must match the corresponding command segment by prefix
	// or glob. A single-segment allow rule can never open a chained command.
	patternSegments := splitCommandSegments(pattern)
	commandSegments := splitCommandSegments(command)
	if len(patternSegments) != len(commandSegments) {
		return false
	}
	for index := range patternSegments {
		patternSegment := strings.TrimSpace(patternSegments[index])
		commandSegment := strings.TrimSpace(commandSegments[index])
		if patternSegment == "" || commandSegment == "" {
			return false
		}
		if !strings.HasPrefix(commandSegment, patternSegment) &&
			!matchGlob(patternSegment, commandSegment) {
			return false
		}
	}
	return true
}

func bashSegmentMatches(pattern, command string) bool {
	for _, segment := range splitCommandSegments(command) {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if strings.HasPrefix(segment, pattern) || matchGlob(pattern, segment) {
			return true
		}
	}
	return false
}

func pathUnderOwned(ownedPaths []string, path string) bool {
	for _, owned := range ownedPaths {
		if matchPathGlob(owned, path) {
			return true
		}
	}
	return false
}

func matchPathGlob(pattern, path string) bool {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(strings.TrimPrefix(path, "./"), "/")
	return matchSegments(patternSegments, pathSegments)
}

func matchSegments(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return len(segments) == 0
	}
	if pattern[0] == "**" {
		for index := 0; index <= len(segments); index++ {
			if matchSegments(pattern[1:], segments[index:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	if !matchSingle(pattern[0], segments[0]) {
		return false
	}
	return matchSegments(pattern[1:], segments[1:])
}

func matchSingle(pattern, value string) bool {
	if pattern == "" {
		return value == ""
	}
	if pattern[0] == '*' {
		for index := 0; index <= len(value); index++ {
			if matchSingle(pattern[1:], value[index:]) {
				return true
			}
		}
		return false
	}
	if value == "" {
		return false
	}
	if pattern[0] == '?' || pattern[0] == value[0] {
		return matchSingle(pattern[1:], value[1:])
	}
	return false
}

func matchGlob(pattern, value string) bool {
	return matchSingle(pattern, value)
}
