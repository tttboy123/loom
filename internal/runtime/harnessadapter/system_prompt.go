package harnessadapter

import (
	"loom-pi-rebuild/internal/prompting"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func buildHarnessSystemPrompt(
	binding loomruntime.FrozenExecutionBinding,
) (string, error) {
	tools := make([]prompting.ToolCapability, 0, 1)
	switch binding.HarnessAdapter {
	case ClaudeCodeAdapterType, CodexAdapterType:
		if containsHarnessCapability(binding.Capabilities, "workspace_edit") ||
			containsHarnessCapability(
				binding.Capabilities,
				loomruntime.CapabilityContextRetrieval,
			) {
			tools = append(tools, prompting.ToolMCP)
		}
	}
	return prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeAgent, ProviderID: binding.ProviderID,
		ModelID: binding.ModelID, HarnessAdapter: binding.HarnessAdapter,
		Tools: tools,
	})
}
