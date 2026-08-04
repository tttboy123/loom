package tui

import (
	"strings"
	"testing"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/observability"
)

func TestRenderIntegrationViewShowsState(t *testing.T) {
	model := Model{
		integrationClient: &fakeReadClient{},
		integrationSnapshot: app.IntegrationSnapshot{
			ViewVersion: "sf3-view",
			Releases: []integration.ReleaseCandidate{{
				ReleaseID: "release-1", TargetBranch: "main", Status: "published",
			}},
			Canaries: []integration.CanaryRun{{
				CanaryID: "canary-1", Status: "completed",
			}},
			Attention: []observability.AttentionItem{{
				Kind: "human_required", SourceID: "attempt-1", Lane: "human",
			}},
		},
	}
	view := model.renderIntegrationView()
	if !strings.Contains(view, "release-1") ||
		!strings.Contains(view, "canary-1") ||
		!strings.Contains(view, "attempt-1") {
		t.Fatalf("integration view = %q", view)
	}
}
