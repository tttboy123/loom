package mode

type Mode string

const (
	ModeConversation Mode = "conversation"
	ModeAgent        Mode = "agent"
)

type Trigger string

const (
	TriggerPlainInput  Trigger = "plain_input"
	TriggerUseAgent    Trigger = "use_agent"
	TriggerSelectAgent Trigger = "select_agent"
	TriggerSelectTeam  Trigger = "select_team"
	TriggerAssign      Trigger = "assign"
)

type Intent struct {
	Mode     Mode
	Trigger  Trigger
	TargetID string
	Text     string
}

type Decision struct {
	Mode Mode
}

func Route(intent Intent) Decision {
	switch intent.Trigger {
	case TriggerUseAgent, TriggerSelectAgent, TriggerSelectTeam, TriggerAssign:
		return Decision{Mode: ModeAgent}
	default:
		return Decision{Mode: ModeConversation}
	}
}
