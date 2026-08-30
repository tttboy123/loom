package main

import (
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/prompting"
)

func productConversationControlSystemPrompt(
	providerID string,
	modelID string,
	harnessAdapter string,
	registry *controltool.Registry,
) (string, error) {
	var controlTools []string
	if registry != nil {
		definitions := registry.Definitions()
		controlTools = make([]string, 0, len(definitions))
		for _, definition := range definitions {
			controlTools = append(controlTools, definition.MCPName)
		}
	}
	return prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeConversation, ProviderID: providerID,
		ModelID: modelID, HarnessAdapter: harnessAdapter,
		ControlTools: controlTools,
	})
}
