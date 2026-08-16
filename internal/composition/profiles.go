package composition

var builtInProfileBundleIDs = []string{
	"loom-agent-runtime",
	"loom-assets",
	"loom-conversation",
	"loom-core",
	"loom-governance",
	"loom-local-ipc",
	"loom-observability",
	"loom-vault",
	"loom-work",
}

func BuiltInLaunchProfile(id ProfileID) (LaunchProfile, error) {
	profile := LaunchProfile{
		SchemaVersion:          1,
		ID:                     id,
		UnavailableRoutePolicy: RouteUnavailableFailStartup,
		ShutdownTimeoutMillis:  15_000,
		DiagnosticMode:         "persistent-local",
		ProtectedCapabilities:  ProtectedCapabilities(),
	}
	for _, bundleID := range builtInProfileBundleIDs {
		profile.Bundles = append(profile.Bundles, ProfileBundle{
			ID: bundleID, Version: builtInBundleVersions[bundleID], Mandatory: true,
		})
	}
	switch id {
	case ProfileDesktop:
		profile.NativeUIRequired = true
		profile.FeatureFlags = []string{"native-projections"}
	case ProfileHeadless:
		profile.FeatureFlags = []string{"headless"}
	case ProfileTest:
		profile.ShutdownTimeoutMillis = 5_000
		profile.DiagnosticMode = "memory-only"
		profile.FeatureFlags = []string{"injected-dependencies"}
	default:
		return LaunchProfile{}, ErrInvalidComposition
	}
	return cloneProfile(profile), nil
}
