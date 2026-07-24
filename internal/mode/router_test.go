package mode

import "testing"

func TestRoutePlainInputUsesConversationMode(t *testing.T) {
	intent := Intent{
		Trigger: TriggerPlainInput,
		Text:    "explain the current work",
	}

	decision := Route(intent)

	if decision.Mode != ModeConversation {
		t.Fatalf("Route() mode = %q, want %q", decision.Mode, ModeConversation)
	}
}

func TestRouteExplicitAgentTriggersUseAgentMode(t *testing.T) {
	tests := []struct {
		name    string
		trigger Trigger
	}{
		{name: "use agent", trigger: TriggerUseAgent},
		{name: "select agent", trigger: TriggerSelectAgent},
		{name: "select team", trigger: TriggerSelectTeam},
		{name: "assign", trigger: TriggerAssign},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := Intent{Trigger: tt.trigger}

			decision := Route(intent)

			if decision.Mode != ModeAgent {
				t.Fatalf("Route() mode = %q, want %q", decision.Mode, ModeAgent)
			}
		})
	}
}

func TestRouteUnknownEmptyOrAmbiguousTriggersStayConversation(t *testing.T) {
	tests := []struct {
		name    string
		trigger Trigger
	}{
		{name: "unknown", trigger: "open_agent_if_useful"},
		{name: "empty", trigger: ""},
		{name: "ambiguous", trigger: "maybe_agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := Intent{Trigger: tt.trigger}

			decision := Route(intent)

			if decision.Mode != ModeConversation {
				t.Fatalf("Route() mode = %q, want %q", decision.Mode, ModeConversation)
			}
		})
	}
}
