package composition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"
)

type CompileInput struct {
	Profile    LaunchProfile
	Bundles    []Bundle
	Context    context.Context
	IncidentID string
	Recorder   DiagnosticRecorder
}

type Plan struct {
	snapshot Snapshot
	bundles  []Bundle
}

func (plan *Plan) Snapshot() Snapshot {
	if plan == nil {
		return Snapshot{}
	}
	return plan.snapshot.Clone()
}

func Compile(input CompileInput) (*Plan, error) {
	started := time.Now()
	plan, err := compile(input)
	recordCompileDiagnostic(input, plan, StageCompositionCompile, started, err)
	if err == nil {
		recordCompileDiagnostic(input, plan, StageCompositionValidate, started, nil)
		recordCompileDiagnostic(input, plan, StageRouteCompile, started, nil)
	}
	return plan, err
}

func compile(input CompileInput) (*Plan, error) {
	if !nilInterface(input.Recorder) &&
		(input.Context == nil || !validIdentifier(input.IncidentID)) {
		return nil, ErrInvalidComposition
	}
	profile, err := validateAndNormalizeProfile(input.Profile)
	if err != nil || len(input.Bundles) == 0 {
		return nil, errors.Join(ErrInvalidComposition, err)
	}
	bundlesByID := make(map[string]Bundle, len(input.Bundles))
	descriptorsByID := make(map[string]BundleDescriptor, len(input.Bundles))
	providerByCapability := make(map[CapabilityRef]string)
	for _, bundle := range input.Bundles {
		if nilInterface(bundle) {
			return nil, ErrInvalidComposition
		}
		descriptor, descriptorErr := validateAndNormalizeDescriptor(bundle.Descriptor())
		if descriptorErr != nil {
			return nil, errors.Join(ErrInvalidComposition, descriptorErr)
		}
		if _, exists := bundlesByID[descriptor.ID]; exists {
			return nil, ErrCompositionConflict
		}
		bundlesByID[descriptor.ID] = bundle
		descriptorsByID[descriptor.ID] = descriptor
		for _, capability := range descriptor.Provides {
			if owner, exists := providerByCapability[capability]; exists && owner != descriptor.ID {
				return nil, ErrCompositionConflict
			}
			if capabilityProtected(capability) &&
				(!descriptor.CoreProtected || descriptor.ID != "loom-core") {
				return nil, ErrCapabilityProtected
			}
			providerByCapability[capability] = descriptor.ID
		}
	}
	if err := validateProfileSelection(profile, descriptorsByID); err != nil {
		return nil, err
	}
	if err := validateCompatibility(descriptorsByID); err != nil {
		return nil, err
	}
	orderedIDs, err := topologicalBundleOrder(descriptorsByID, providerByCapability)
	if err != nil {
		return nil, err
	}
	orderedBundles := make([]Bundle, len(orderedIDs))
	orderedDescriptors := make([]BundleDescriptor, len(orderedIDs))
	for index, id := range orderedIDs {
		orderedBundles[index] = bundlesByID[id]
		orderedDescriptors[index] = descriptorsByID[id]
	}
	routes, err := compileRoutes(orderedDescriptors, providerByCapability, profile)
	if err != nil {
		return nil, err
	}
	canonicalInput := struct {
		SchemaVersion int                `json:"schema_version"`
		Profile       LaunchProfile      `json:"profile"`
		Bundles       []BundleDescriptor `json:"bundles"`
		Routes        []RouteDescriptor  `json:"routes"`
	}{1, profile, orderedDescriptors, routes}
	canonical, err := json.Marshal(canonicalInput)
	if err != nil {
		return nil, errors.Join(ErrInvalidComposition, err)
	}
	digestBytes := sha256.Sum256(canonical)
	snapshot := Snapshot{
		SchemaVersion: 1, Profile: profile, Bundles: orderedDescriptors, Routes: routes,
		Digest: hex.EncodeToString(digestBytes[:]), Canonical: canonical,
	}
	return &Plan{snapshot: snapshot, bundles: orderedBundles}, nil
}

func validateAndNormalizeProfile(input LaunchProfile) (LaunchProfile, error) {
	profile := cloneProfile(input)
	if profile.SchemaVersion != 1 ||
		profile.ID != ProfileDesktop && profile.ID != ProfileHeadless && profile.ID != ProfileTest ||
		len(profile.Bundles) == 0 || profile.ShutdownTimeoutMillis < 1 ||
		profile.ShutdownTimeoutMillis > 300_000 ||
		profile.UnavailableRoutePolicy != RouteUnavailableFailStartup &&
			profile.UnavailableRoutePolicy != RouteUnavailableOmit {
		return LaunchProfile{}, ErrInvalidComposition
	}
	sort.Slice(profile.Bundles, func(left, right int) bool {
		if profile.Bundles[left].ID != profile.Bundles[right].ID {
			return profile.Bundles[left].ID < profile.Bundles[right].ID
		}
		return profile.Bundles[left].Version < profile.Bundles[right].Version
	})
	for index, bundle := range profile.Bundles {
		if !validIdentifier(bundle.ID) || !validVersion(bundle.Version) ||
			index > 0 && profile.Bundles[index-1].ID == bundle.ID {
			return LaunchProfile{}, ErrInvalidComposition
		}
	}
	sort.Slice(profile.RequiredRoutes, func(left, right int) bool {
		return profile.RequiredRoutes[left] < profile.RequiredRoutes[right]
	})
	if !validUniqueRouteMethods(profile.RequiredRoutes) {
		return LaunchProfile{}, ErrInvalidComposition
	}
	sortCapabilityRefs(profile.ProtectedCapabilities)
	if !reflectCapabilityRefs(profile.ProtectedCapabilities, ProtectedCapabilities()) {
		return LaunchProfile{}, ErrInvalidComposition
	}
	sort.Strings(profile.FeatureFlags)
	if !validUniqueIdentifiers(profile.FeatureFlags) ||
		profile.DiagnosticMode != "" && !validIdentifier(profile.DiagnosticMode) {
		return LaunchProfile{}, ErrInvalidComposition
	}
	return profile, nil
}

func validateAndNormalizeDescriptor(input BundleDescriptor) (BundleDescriptor, error) {
	descriptor := cloneDescriptors([]BundleDescriptor{input})[0]
	wantedVersion, knownBundle := builtInBundleVersions[descriptor.ID]
	if descriptor.SchemaVersion != 1 || !knownBundle || descriptor.Version != wantedVersion ||
		!validIdentifier(descriptor.ID) || !validVersion(descriptor.Version) ||
		!validIdentifier(descriptor.LifecycleID) ||
		descriptor.CoreProtected != (descriptor.ID == "loom-core") {
		return BundleDescriptor{}, ErrInvalidComposition
	}
	for _, set := range [][]CapabilityRef{
		descriptor.Requires, descriptor.OptionalRequires, descriptor.Provides,
	} {
		for _, ref := range set {
			if !validCapabilityRef(ref) {
				return BundleDescriptor{}, ErrInvalidComposition
			}
		}
	}
	sortCapabilityRefs(descriptor.Requires)
	sortCapabilityRefs(descriptor.OptionalRequires)
	sortCapabilityRefs(descriptor.Provides)
	if !validUniqueCapabilities(descriptor.Requires) ||
		!validUniqueCapabilities(descriptor.OptionalRequires) ||
		!validUniqueCapabilities(descriptor.Provides) ||
		capabilitySetsOverlap(descriptor.Requires, descriptor.OptionalRequires) ||
		capabilitySetsOverlap(descriptor.Requires, descriptor.Provides) ||
		capabilitySetsOverlap(descriptor.OptionalRequires, descriptor.Provides) {
		return BundleDescriptor{}, ErrInvalidComposition
	}
	for _, ref := range append(
		append([]CapabilityRef(nil), descriptor.Requires...), descriptor.OptionalRequires...,
	) {
		if capabilityProtected(ref) && !descriptor.CoreProtected {
			return BundleDescriptor{}, ErrCapabilityProtected
		}
	}
	sort.Slice(descriptor.Compatibility, func(left, right int) bool {
		return descriptor.Compatibility[left].BundleID < descriptor.Compatibility[right].BundleID
	})
	for index, constraint := range descriptor.Compatibility {
		if !validIdentifier(constraint.BundleID) || !validVersion(constraint.Version) ||
			index > 0 && descriptor.Compatibility[index-1].BundleID == constraint.BundleID {
			return BundleDescriptor{}, ErrInvalidComposition
		}
	}
	for index := range descriptor.Routes {
		route, err := validateAndNormalizeRoute(descriptor.Routes[index], descriptor)
		if err != nil {
			return BundleDescriptor{}, err
		}
		descriptor.Routes[index] = route
	}
	sort.Slice(descriptor.Routes, func(left, right int) bool {
		return descriptor.Routes[left].Method < descriptor.Routes[right].Method
	})
	if !validRouteSet(descriptor.Routes) {
		return BundleDescriptor{}, ErrInvalidComposition
	}
	return descriptor, nil
}

func validateAndNormalizeRoute(
	input RouteDescriptor,
	owner BundleDescriptor,
) (RouteDescriptor, error) {
	route := cloneRoutes([]RouteDescriptor{input})[0]
	if route.SchemaVersion != 1 || !validIdentifier(string(route.Method)) ||
		route.OwnerBundle != owner.ID || !validCapabilityRef(route.HandlerCapability) ||
		route.PrivacyClass != PrivacyMetadataOnly && route.PrivacyClass != PrivacyLocalContent ||
		protectedRoutes[route.Method] && (!owner.CoreProtected || owner.ID != "loom-core") ||
		route.AvailabilityFailure != "" && !validIdentifier(route.AvailabilityFailure) ||
		route.IncidentPolicy != "" && !validIdentifier(route.IncidentPolicy) {
		return RouteDescriptor{}, ErrInvalidComposition
	}
	if capabilityProtected(route.HandlerCapability) && !owner.CoreProtected {
		return RouteDescriptor{}, ErrCapabilityProtected
	}
	declared := append(
		append(append([]CapabilityRef(nil), owner.Requires...), owner.OptionalRequires...),
		owner.Provides...,
	)
	if !containsCapability(declared, route.HandlerCapability) {
		return RouteDescriptor{}, ErrCapabilityUnavailable
	}
	sortCapabilityRefs(route.RequiredCapabilities)
	if !validUniqueCapabilities(route.RequiredCapabilities) {
		return RouteDescriptor{}, ErrInvalidComposition
	}
	for _, ref := range route.RequiredCapabilities {
		if !validCapabilityRef(ref) || capabilityProtected(ref) && !owner.CoreProtected ||
			!containsCapability(declared, ref) {
			return RouteDescriptor{}, ErrInvalidComposition
		}
	}
	return route, nil
}

func validateProfileSelection(
	profile LaunchProfile,
	descriptors map[string]BundleDescriptor,
) error {
	selected := make(map[string]ProfileBundle, len(profile.Bundles))
	for _, bundle := range profile.Bundles {
		selected[bundle.ID] = bundle
		descriptor, found := descriptors[bundle.ID]
		if bundle.Mandatory && !found || found && descriptor.Version != bundle.Version {
			return ErrInvalidComposition
		}
	}
	for id := range descriptors {
		if _, found := selected[id]; !found {
			return ErrInvalidComposition
		}
	}
	return nil
}

func validateCompatibility(descriptors map[string]BundleDescriptor) error {
	for _, descriptor := range descriptors {
		for _, constraint := range descriptor.Compatibility {
			dependency, ok := descriptors[constraint.BundleID]
			if !ok || dependency.Version != constraint.Version {
				return ErrInvalidComposition
			}
		}
	}
	return nil
}

func topologicalBundleOrder(
	descriptors map[string]BundleDescriptor,
	providers map[CapabilityRef]string,
) ([]string, error) {
	dependencies := make(map[string]map[string]bool, len(descriptors))
	reverse := make(map[string][]string, len(descriptors))
	indegree := make(map[string]int, len(descriptors))
	for id, descriptor := range descriptors {
		dependencies[id] = make(map[string]bool)
		for _, required := range descriptor.Requires {
			provider, found := providers[required]
			if !found || provider == id {
				return nil, ErrInvalidComposition
			}
			dependencies[id][provider] = true
		}
		for _, optional := range descriptor.OptionalRequires {
			if provider, found := providers[optional]; found && provider != id {
				dependencies[id][provider] = true
			}
		}
		indegree[id] = len(dependencies[id])
		for dependency := range dependencies[id] {
			reverse[dependency] = append(reverse[dependency], id)
		}
	}
	ready := make([]string, 0)
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	ordered := make([]string, 0, len(descriptors))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, id)
		dependents := append([]string(nil), reverse[id]...)
		sort.Strings(dependents)
		for _, dependent := range dependents {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(descriptors) {
		return nil, ErrCompositionConflict
	}
	return ordered, nil
}

func compileRoutes(
	descriptors []BundleDescriptor,
	providers map[CapabilityRef]string,
	profile LaunchProfile,
) ([]RouteDescriptor, error) {
	routes := make([]RouteDescriptor, 0)
	seen := make(map[RouteMethod]string)
	for _, descriptor := range descriptors {
		for _, route := range descriptor.Routes {
			if owner, exists := seen[route.Method]; exists && owner != descriptor.ID {
				return nil, ErrCompositionConflict
			}
			if _, found := providers[route.HandlerCapability]; !found {
				return nil, ErrCapabilityUnavailable
			}
			for _, required := range route.RequiredCapabilities {
				if _, found := providers[required]; !found {
					return nil, ErrCapabilityUnavailable
				}
			}
			seen[route.Method] = descriptor.ID
			routes = append(routes, route)
		}
	}
	for _, required := range profile.RequiredRoutes {
		if _, found := seen[required]; !found &&
			profile.UnavailableRoutePolicy == RouteUnavailableFailStartup {
			return nil, ErrCapabilityUnavailable
		}
	}
	sort.Slice(routes, func(left, right int) bool { return routes[left].Method < routes[right].Method })
	return routes, nil
}

func validUniqueCapabilities(values []CapabilityRef) bool {
	for index, value := range values {
		if !validCapabilityRef(value) || index > 0 && values[index-1] == value {
			return false
		}
	}
	return true
}

func capabilitySetsOverlap(left, right []CapabilityRef) bool {
	seen := make(map[CapabilityRef]bool, len(left))
	for _, value := range left {
		seen[value] = true
	}
	for _, value := range right {
		if seen[value] {
			return true
		}
	}
	return false
}

func reflectCapabilityRefs(left, right []CapabilityRef) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validUniqueRouteMethods(values []RouteMethod) bool {
	for index, value := range values {
		if !validIdentifier(string(value)) || index > 0 && values[index-1] == value {
			return false
		}
	}
	return true
}

func validRouteSet(values []RouteDescriptor) bool {
	for index := range values {
		if index > 0 && values[index-1].Method == values[index].Method {
			return false
		}
	}
	return true
}

func validUniqueIdentifiers(values []string) bool {
	for index, value := range values {
		if !validIdentifier(value) || index > 0 && values[index-1] == value {
			return false
		}
	}
	return true
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func compositionError(stage string, bundle string, err error) error {
	category := ErrInvalidComposition
	if errors.Is(err, ErrCapabilityProtected) {
		category = ErrCapabilityProtected
	} else if errors.Is(err, ErrCapabilityUnavailable) {
		category = ErrCapabilityUnavailable
	} else if errors.Is(err, ErrCompositionConflict) {
		category = ErrCompositionConflict
	}
	return &lifecycleFailure{
		stage: stage, bundle: bundle, category: category, cause: err,
	}
}

type lifecycleFailure struct {
	stage    string
	bundle   string
	category error
	cause    error
}

func (failure *lifecycleFailure) Error() string {
	if failure == nil {
		return "composition failed"
	}
	return fmt.Sprintf(
		"composition failed: stage=%s bundle=%s", failure.stage, failure.bundle,
	)
}

func (failure *lifecycleFailure) Unwrap() []error {
	if failure == nil {
		return nil
	}
	return []error{failure.category, failure.cause}
}

func recordCompileDiagnostic(
	input CompileInput,
	plan *Plan,
	stage DiagnosticStage,
	started time.Time,
	err error,
) {
	if nilInterface(input.Recorder) || input.Context == nil || !validIdentifier(input.IncidentID) {
		return
	}
	result, code := "succeeded", ""
	if err != nil {
		result, code = "failed", "composition_failed"
	}
	digest := ""
	if plan != nil {
		digest = plan.snapshot.Digest
	}
	_ = input.Recorder.RecordCompositionDiagnostic(input.Context, DiagnosticRecord{
		IncidentID: input.IncidentID, ProfileID: input.Profile.ID,
		SnapshotDigest: digest, Stage: stage,
		ElapsedMillis: time.Since(started).Milliseconds(), Result: result,
		ErrorCode: code, Retryable: false,
	})
}
