package composition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type testPort interface{ Name() string }

type testPortValue struct{ name string }

func (value *testPortValue) Name() string { return value.name }

type testBundle struct {
	descriptor             BundleDescriptor
	portKey                CapabilityKey[testPort]
	port                   testPort
	requireKey             CapabilityKey[testPort]
	events                 *[]string
	fail                   string
	failErr                error
	onStart                func(*BundleContext) error
	partialEffectOnFailure bool
}

func (bundle *testBundle) Descriptor() BundleDescriptor { return bundle.descriptor }

func (bundle *testBundle) Register(_ context.Context, registration *Registration) (Effect, error) {
	*bundle.events = append(*bundle.events, "register:"+bundle.descriptor.ID)
	if bundle.fail == "register" {
		if bundle.partialEffectOnFailure {
			return NewEffect(func(context.Context) error {
				*bundle.events = append(*bundle.events, "dispose-partial-register:"+bundle.descriptor.ID)
				return nil
			}), bundle.failErr
		}
		if bundle.failErr != nil {
			return nil, bundle.failErr
		}
		return nil, errors.New("register failed")
	}
	if bundle.port != nil {
		if err := Provide(registration, bundle.portKey, bundle.port); err != nil {
			return nil, err
		}
	}
	if bundle.requireKey.Ref().ID != "" {
		if _, err := Require(registration, bundle.requireKey); err != nil {
			return nil, err
		}
	}
	return NewEffect(func(context.Context) error {
		*bundle.events = append(*bundle.events, "dispose-register:"+bundle.descriptor.ID)
		return nil
	}), nil
}

func (bundle *testBundle) Start(_ context.Context, capabilities *BundleContext) (Effect, error) {
	*bundle.events = append(*bundle.events, "start:"+bundle.descriptor.ID)
	if bundle.onStart != nil {
		if err := bundle.onStart(capabilities); err != nil {
			return nil, err
		}
	}
	if bundle.fail == "start" {
		if bundle.partialEffectOnFailure {
			return NewEffect(func(context.Context) error {
				*bundle.events = append(*bundle.events, "dispose-partial-start:"+bundle.descriptor.ID)
				return nil
			}), bundle.failErr
		}
		if bundle.failErr != nil {
			return nil, bundle.failErr
		}
		return nil, errors.New("start failed")
	}
	return NewEffect(func(context.Context) error {
		*bundle.events = append(*bundle.events, "dispose-start:"+bundle.descriptor.ID)
		return nil
	}), nil
}

func (bundle *testBundle) Ready(context.Context, *BundleContext) error {
	*bundle.events = append(*bundle.events, "ready:"+bundle.descriptor.ID)
	if bundle.fail == "ready" {
		if bundle.failErr != nil {
			return bundle.failErr
		}
		return errors.New("ready failed")
	}
	return nil
}

func (bundle *testBundle) Stop(context.Context, *BundleContext) error {
	*bundle.events = append(*bundle.events, "stop:"+bundle.descriptor.ID)
	return nil
}

func TestCOMP1CompilerIsDeterministicAcrossInputOrder(t *testing.T) {
	portKey := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, portKey)
	profile := testProfile(provider.descriptor, consumer.descriptor)

	one, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{consumer, provider}})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider, consumer}})
	if err != nil {
		t.Fatal(err)
	}
	if one.Snapshot().Digest == "" || one.Snapshot().Digest != two.Snapshot().Digest ||
		!bytes.Equal(one.Snapshot().Canonical, two.Snapshot().Canonical) ||
		!reflect.DeepEqual(one.Snapshot().Bundles, two.Snapshot().Bundles) ||
		one.Snapshot().Bundles[0].ID != "loom-observability" {
		t.Fatalf("snapshot one=%#v two=%#v", one.Snapshot(), two.Snapshot())
	}
}

func TestCOMP1CompilerRejectsInvalidGraphAndProtectedOverrides(t *testing.T) {
	port := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	protected := MustCapabilityKey[testPort](CapabilityJournalAppendAuthority)
	events := []string{}
	base := BundleDescriptor{
		SchemaVersion: 1, ID: "loom-observability", Version: "1.0.0",
		LifecycleID: "observability-v1",
	}
	tests := []struct {
		name    string
		profile LaunchProfile
		bundles []Bundle
	}{
		{
			name: "missing dependency",
			bundles: []Bundle{&testBundle{descriptor: BundleDescriptor{
				SchemaVersion: 1, ID: "loom-agent-runtime", Version: "1.0.0",
				LifecycleID: "runtime-v1", Requires: []CapabilityRef{port.Ref()},
			}, requireKey: port, events: &events}},
		},
		{
			name: "cycle",
			bundles: []Bundle{
				&testBundle{descriptor: BundleDescriptor{
					SchemaVersion: 1, ID: "loom-assets", Version: "1.0.0",
					LifecycleID: "assets-v1", Provides: []CapabilityRef{CapabilityAssetsReader.Ref()},
					Requires: []CapabilityRef{port.Ref()},
				}, events: &events},
				&testBundle{descriptor: BundleDescriptor{
					SchemaVersion: 1, ID: "loom-observability", Version: "1.0.0",
					LifecycleID: "observability-v1", Provides: []CapabilityRef{port.Ref()},
					Requires: []CapabilityRef{CapabilityAssetsReader.Ref()},
				}, events: &events},
			},
		},
		{
			name: "duplicate capability",
			bundles: []Bundle{
				&testBundle{descriptor: withProvides(base, port.Ref()), events: &events},
				&testBundle{descriptor: withProvides(BundleDescriptor{
					SchemaVersion: 1, ID: "loom-assets", Version: "1.0.0",
					LifecycleID: "assets-v1",
				}, port.Ref()), events: &events},
			},
		},
		{
			name:    "protected provider is not core",
			bundles: []Bundle{&testBundle{descriptor: withProvides(base, protected.Ref()), events: &events}},
		},
		{
			name: "protected route override",
			bundles: []Bundle{&testBundle{descriptor: BundleDescriptor{
				SchemaVersion: 1, ID: "loom-observability", Version: "1.0.0",
				LifecycleID: "observability-v1",
				Routes: []RouteDescriptor{{
					Method: RouteJournalAppend, SchemaVersion: 1,
					OwnerBundle: "loom-observability", HandlerCapability: port.Ref(),
					PrivacyClass: PrivacyMetadataOnly,
				}},
			}, events: &events}},
		},
		{
			name: "route uses undeclared handler capability",
			bundles: []Bundle{
				&testBundle{descriptor: withProvides(base, port.Ref()), portKey: port,
					port: &testPortValue{name: "root-observability"}, events: &events},
				&testBundle{descriptor: BundleDescriptor{
					SchemaVersion: 1, ID: "loom-agent-runtime", Version: "1.0.0",
					LifecycleID: "runtime-v1", Routes: []RouteDescriptor{{
						Method: "agent.dispatch", SchemaVersion: 1, OwnerBundle: "loom-agent-runtime",
						HandlerCapability: port.Ref(), PrivacyClass: PrivacyMetadataOnly,
					}},
				}, events: &events},
			},
		},
		{
			name: "loom core omits protected marker",
			bundles: []Bundle{&testBundle{descriptor: BundleDescriptor{
				SchemaVersion: 1, ID: "loom-core", Version: "1.0.0", LifecycleID: "loom-core-v1",
			}, events: &events}},
		},
		{
			name: "duplicate route",
			bundles: []Bundle{
				&testBundle{descriptor: BundleDescriptor{
					SchemaVersion: 1, ID: "loom-observability", Version: "1.0.0",
					LifecycleID: "observability-v1", Provides: []CapabilityRef{port.Ref()},
					Routes: []RouteDescriptor{{
						Method: "agent.dispatch", SchemaVersion: 1, OwnerBundle: "loom-observability",
						HandlerCapability: port.Ref(), PrivacyClass: PrivacyMetadataOnly,
					}},
				}, portKey: port, port: &testPortValue{name: "observability"}, events: &events},
				&testBundle{descriptor: BundleDescriptor{
					SchemaVersion: 1, ID: "loom-assets", Version: "1.0.0",
					LifecycleID: "assets-v1", Provides: []CapabilityRef{CapabilityAssetsReader.Ref()},
					Routes: []RouteDescriptor{{
						Method: "agent.dispatch", SchemaVersion: 1, OwnerBundle: "loom-assets",
						HandlerCapability: CapabilityAssetsReader.Ref(), PrivacyClass: PrivacyMetadataOnly,
					}},
				}, portKey: MustCapabilityKey[testPort](CapabilityAssetsReader),
					port: &testPortValue{name: "assets"}, events: &events},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := test.profile
			if profile.ID == "" {
				descriptors := make([]BundleDescriptor, len(test.bundles))
				for index, bundle := range test.bundles {
					descriptors[index] = bundle.Descriptor()
				}
				profile = testProfile(descriptors...)
			}
			if _, err := Compile(CompileInput{Profile: profile, Bundles: test.bundles}); err == nil {
				t.Fatal("invalid composition compiled")
			}
			if len(events) != 0 {
				t.Fatalf("compile started bundle work: %v", events)
			}
		})
	}
}

func TestCOMP1ActivationRollsBackInReverseAndPublishesReadyAtomically(t *testing.T) {
	portKey := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, portKey)
	consumer.fail = "start"
	plan, err := Compile(CompileInput{
		Profile: testProfile(provider.descriptor, consumer.descriptor),
		Bundles: []Bundle{consumer, provider},
	})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := plan.Activate(context.Background(), "incident-comp-1", nil)
	if err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	want := []string{
		"register:loom-observability", "register:loom-agent-runtime",
		"start:loom-observability", "start:loom-agent-runtime",
		"stop:loom-observability",
		"dispose-start:loom-observability",
		"dispose-register:loom-agent-runtime", "dispose-register:loom-observability",
	}
	if !reflect.DeepEqual(*provider.events, want) {
		t.Fatalf("lifecycle=%v want=%v", *provider.events, want)
	}

	*provider.events = nil
	consumer.fail = ""
	plan, err = Compile(CompileInput{
		Profile: testProfile(provider.descriptor, consumer.descriptor),
		Bundles: []Bundle{consumer, provider},
	})
	if err != nil {
		t.Fatal(err)
	}
	activation, err = plan.Activate(context.Background(), "incident-comp-2", nil)
	if err != nil || !activation.Ready() || activation.SnapshotDigest() != plan.Snapshot().Digest {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if err := activation.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := activation.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	want = []string{
		"register:loom-observability", "register:loom-agent-runtime",
		"start:loom-observability", "start:loom-agent-runtime",
		"ready:loom-observability", "ready:loom-agent-runtime",
		"stop:loom-observability", "stop:loom-agent-runtime",
		"dispose-start:loom-agent-runtime", "dispose-start:loom-observability",
		"dispose-register:loom-agent-runtime", "dispose-register:loom-observability",
	}
	if !reflect.DeepEqual(*provider.events, want) {
		t.Fatalf("lifecycle=%v want=%v", *provider.events, want)
	}
}

func TestCOMP1ScopedContextNarrowsAndFreezesAttemptSnapshot(t *testing.T) {
	portKey := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, portKey)
	plan, err := Compile(CompileInput{
		Profile: testProfile(provider.descriptor, consumer.descriptor),
		Bundles: []Bundle{provider, consumer},
	})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := plan.Activate(context.Background(), "incident-scope", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = activation.Close(context.Background()) }()
	product, err := activation.Root().OpenChild(context.Background(), ScopeProduct, ScopeOptions{ID: "product-desktop"})
	if err != nil {
		t.Fatal(err)
	}
	conversation := mustChild(t, product, ScopeConversation, "conversation-1", ScopeOptions{})
	team := mustChild(t, conversation, ScopeTeam, "team-1", ScopeOptions{})
	agent := mustChild(t, team, ScopeAgent, "agent-1", ScopeOptions{})
	attempt := mustChild(t, agent, ScopeAttempt, "attempt-1", ScopeOptions{
		CompositionSnapshotDigest: plan.Snapshot().Digest,
		ExecutionBindingDigest:    repeatHex("a"),
	})
	turn := mustChild(t, attempt, ScopeTurn, "turn-1", ScopeOptions{Generation: 1})

	rootValue, err := Lookup(activation.Root(), portKey)
	if err != nil || rootValue.Name() != "root-observability" {
		t.Fatalf("root lookup=%#v err=%v", rootValue, err)
	}
	maskedProduct := mustChild(t, activation.Root(), ScopeProduct, "product-masked", ScopeOptions{})
	if maskedProduct.Kind() != ScopeProduct || maskedProduct.ID() != "product-masked" ||
		maskedProduct.Digest() == "" || maskedProduct.CompositionSnapshotDigest() != plan.Snapshot().Digest {
		t.Fatalf("masked product identity=%#v", maskedProduct)
	}
	if err := maskedProduct.Mask(portKey.Ref()); err != nil {
		t.Fatal(err)
	}
	if _, err := Lookup(maskedProduct, portKey); err == nil {
		t.Fatal("masked capability remained visible")
	}
	if err := maskedProduct.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	narrowed := &testPortValue{name: "turn-observability"}
	if err := BindScoped(turn, portKey, testPort(narrowed)); err != nil {
		t.Fatal(err)
	}
	turnValue, err := Lookup(turn, portKey)
	if err != nil || turnValue.Name() != narrowed.Name() {
		t.Fatalf("turn lookup=%#v err=%v", turnValue, err)
	}
	unprovided := MustCapabilityKey[testPort](CapabilityAssetsReader)
	if err := BindScoped(turn, unprovided, testPort(&testPortValue{name: "widened"})); err == nil {
		t.Fatal("child widened an absent capability")
	}
	protected := MustCapabilityKey[testPort](CapabilityJournalAppendAuthority)
	if err := BindScoped(turn, protected, testPort(&testPortValue{name: "forbidden"})); err == nil {
		t.Fatal("child bound protected authority")
	}
	if attempt.CompositionSnapshotDigest() != plan.Snapshot().Digest ||
		attempt.ExecutionBindingDigest() != repeatHex("a") {
		t.Fatalf("attempt binding drifted: %#v", attempt)
	}

	var wait sync.WaitGroup
	for index := 0; index < 16; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_ = turn.Close(context.Background())
		}()
	}
	wait.Wait()
	if _, err := Lookup(turn, portKey); err == nil {
		t.Fatal("closed scope retained capability")
	}
	if _, err := turn.OpenChild(context.Background(), ScopeTurn, ScopeOptions{ID: "turn-2"}); err == nil {
		t.Fatal("closed scope opened a child")
	}
}

func TestCOMP1CanonicalSnapshotRejectsContentAndUnknownVersions(t *testing.T) {
	portKey := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, portKey)
	profile := testProfile(provider.descriptor, consumer.descriptor)
	plan, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider, consumer}})
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range [][]byte{
		[]byte("API_KEY"), []byte("Authorization"), []byte("private prompt"),
		[]byte("provider response"),
	} {
		if bytes.Contains(plan.Snapshot().Canonical, marker) {
			t.Fatalf("snapshot leaked marker %q", marker)
		}
	}
	badProfile := profile
	badProfile.SchemaVersion = 2
	if _, err := Compile(CompileInput{Profile: badProfile, Bundles: []Bundle{provider, consumer}}); err == nil {
		t.Fatal("unknown profile schema compiled")
	}
	provider.descriptor.SchemaVersion = 2
	if _, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider, consumer}}); err == nil {
		t.Fatal("unknown Bundle schema compiled")
	}
	provider.descriptor.SchemaVersion = 1
	provider.descriptor.Version = "1.0.1"
	if _, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider, consumer}}); err == nil {
		t.Fatal("unknown built-in Bundle version compiled")
	}
	if _, err := NewCapabilityKey[any](CapabilityObservabilityRecorder); err == nil {
		t.Fatal("content-capable empty interface key was admitted")
	}
}

func TestCOMP1BuiltInProfilesCompileExactManifests(t *testing.T) {
	wantBundles := []string{
		"loom-agent-runtime", "loom-assets", "loom-conversation", "loom-core",
		"loom-governance", "loom-local-ipc", "loom-observability", "loom-vault", "loom-work",
	}
	tests := []struct {
		id             ProfileID
		nativeUI       bool
		diagnosticMode string
		featureFlags   []string
	}{
		{ProfileDesktop, true, "persistent-local", []string{"native-projections"}},
		{ProfileHeadless, false, "persistent-local", []string{"headless"}},
		{ProfileTest, false, "memory-only", []string{"injected-dependencies"}},
	}
	for _, test := range tests {
		t.Run(string(test.id), func(t *testing.T) {
			profile, err := BuiltInLaunchProfile(test.id)
			if err != nil {
				t.Fatal(err)
			}
			if profile.NativeUIRequired != test.nativeUI ||
				profile.DiagnosticMode != test.diagnosticMode ||
				!reflect.DeepEqual(profile.FeatureFlags, test.featureFlags) {
				t.Fatalf("profile=%#v", profile)
			}
			bundleIDs := make([]string, len(profile.Bundles))
			bundles := make([]Bundle, len(profile.Bundles))
			events := []string{}
			for index, selected := range profile.Bundles {
				bundleIDs[index] = selected.ID
				bundles[index] = &testBundle{descriptor: BundleDescriptor{
					SchemaVersion: 1, ID: selected.ID, Version: selected.Version,
					LifecycleID: selected.ID + "-v1", CoreProtected: selected.ID == "loom-core",
				}, events: &events}
			}
			if !reflect.DeepEqual(bundleIDs, wantBundles) {
				t.Fatalf("bundles=%v want=%v", bundleIDs, wantBundles)
			}
			plan, err := Compile(CompileInput{Profile: profile, Bundles: bundles})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Snapshot().Profile.ID != test.id || len(plan.Snapshot().Routes) != 0 || len(events) != 0 {
				t.Fatalf("snapshot=%#v events=%v", plan.Snapshot(), events)
			}
		})
	}
	if _, err := BuiltInLaunchProfile(ProfileID("unknown")); err == nil {
		t.Fatal("unknown profile was admitted")
	}
}

func TestCOMP1ValidateFailureStartsZeroResources(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	events := []string{}
	bundle := &testBundle{descriptor: BundleDescriptor{
		SchemaVersion: 1, ID: "loom-observability", Version: "1.0.0",
		LifecycleID: "observability-v1", Provides: []CapabilityRef{key.Ref()},
	}, events: &events}
	plan, err := Compile(CompileInput{Profile: testProfile(bundle.descriptor), Bundles: []Bundle{bundle}})
	if err != nil {
		t.Fatal(err)
	}
	if activation, err := plan.Activate(context.Background(), "incident-validate", nil); err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	want := []string{"register:loom-observability", "dispose-register:loom-observability"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%v want=%v", events, want)
	}
}

func TestCOMP1FactoryMustConsumeDeclaredRequiredCapability(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, key)
	consumer.requireKey = CapabilityKey[testPort]{}
	plan, err := Compile(CompileInput{
		Profile: testProfile(provider.descriptor, consumer.descriptor), Bundles: []Bundle{provider, consumer},
	})
	if err != nil {
		t.Fatal(err)
	}
	if activation, err := plan.Activate(context.Background(), "incident-factory-contract", nil); err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	for _, event := range *provider.events {
		if strings.HasPrefix(event, "start:") {
			t.Fatalf("factory mismatch started resources: %v", *provider.events)
		}
	}
}

func TestCOMP1FailedHookDisposesReturnedPartialEffect(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, key)
	consumer.fail = "start"
	consumer.failErr = errors.New("controlled start failure")
	consumer.partialEffectOnFailure = true
	plan, err := Compile(CompileInput{
		Profile: testProfile(provider.descriptor, consumer.descriptor), Bundles: []Bundle{provider, consumer},
	})
	if err != nil {
		t.Fatal(err)
	}
	if activation, err := plan.Activate(context.Background(), "incident-partial-effect", nil); err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	want := []string{
		"register:loom-observability", "register:loom-agent-runtime",
		"start:loom-observability", "start:loom-agent-runtime", "stop:loom-observability",
		"dispose-partial-start:loom-agent-runtime", "dispose-start:loom-observability",
		"dispose-register:loom-agent-runtime", "dispose-register:loom-observability",
	}
	if !reflect.DeepEqual(*provider.events, want) {
		t.Fatalf("events=%v want=%v", *provider.events, want)
	}
}

func TestCOMP1InvalidDiagnosticAdmissionDoesNotCallRecorder(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, key)
	recorder := &recordingDiagnostic{}
	if _, err := Compile(CompileInput{
		Profile: testProfile(provider.descriptor, consumer.descriptor), Bundles: []Bundle{provider, consumer},
		IncidentID: "incident-no-context", Recorder: recorder,
	}); err == nil {
		t.Fatal("diagnostics without context compiled")
	}
	if records := recorder.snapshot(); len(records) != 0 {
		t.Fatalf("invalid diagnostic admission emitted records: %#v", records)
	}
}

func TestCOMP1ProtectedCoreAuthorityIsNotVisibleToBundlesOrProductScopes(t *testing.T) {
	protectedKey := MustCapabilityKey[testPort](CapabilityJournalAppendAuthority)
	events := []string{}
	core := &testBundle{descriptor: BundleDescriptor{
		SchemaVersion: 1, ID: "loom-core", Version: "1.0.0", LifecycleID: "loom-core-v1",
		CoreProtected: true, Provides: []CapabilityRef{protectedKey.Ref()},
	}, portKey: protectedKey, port: &testPortValue{name: "journal-root"}, events: &events}
	consumer := &testBundle{descriptor: BundleDescriptor{
		SchemaVersion: 1, ID: "loom-work", Version: "1.0.0", LifecycleID: "loom-work-v1",
	}, events: &events, onStart: func(capabilities *BundleContext) error {
		if capabilities.BundleID() != "loom-work" ||
			capabilities.CompositionSnapshotDigest() == "" {
			return errors.New("Bundle context identity missing")
		}
		if _, err := LookupBundle(capabilities, protectedKey); err == nil {
			return errors.New("protected authority reached non-core Bundle")
		}
		return nil
	}}
	plan, err := Compile(CompileInput{
		Profile: testProfile(core.descriptor, consumer.descriptor), Bundles: []Bundle{consumer, core},
	})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := plan.Activate(context.Background(), "incident-protected-scope", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = activation.Close(context.Background()) }()
	if value, err := Lookup(activation.Root(), protectedKey); err != nil || value.Name() != "journal-root" {
		t.Fatalf("core root lookup=%#v err=%v", value, err)
	}
	product := mustChild(t, activation.Root(), ScopeProduct, "product-desktop", ScopeOptions{})
	if _, err := Lookup(product, protectedKey); err == nil {
		t.Fatal("protected authority inherited into Product scope")
	}
}

func TestCOMP1OptionalBundleAndUnavailableRoutePolicies(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, _ := deterministicBundles(t, key)
	profile := testProfile(provider.descriptor)
	profile.Bundles = append(profile.Bundles, ProfileBundle{
		ID: "loom-assets", Version: "1.0.0", Mandatory: false,
	})
	profile.RequiredRoutes = []RouteMethod{"agent.dispatch"}
	profile.UnavailableRoutePolicy = RouteUnavailableOmit
	plan, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider}})
	if err != nil || len(plan.Snapshot().Routes) != 0 {
		t.Fatalf("optional route plan=%#v err=%v", plan, err)
	}
	profile.UnavailableRoutePolicy = RouteUnavailableFailStartup
	if _, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider}}); err == nil {
		t.Fatal("missing required route did not fail startup")
	}
	profile.RequiredRoutes = nil
	provider.descriptor.Compatibility = []BundleConstraint{{BundleID: "loom-assets", Version: "1.0.0"}}
	if _, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider}}); err == nil {
		t.Fatal("missing compatible Bundle was admitted")
	}
}

func TestCOMP1DiagnosticsAndScopedEffectsAreSafeAndComplete(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, key)
	recorder := &recordingDiagnostic{recordErr: errors.New("diagnostic sink unavailable")}
	profile := testProfile(provider.descriptor, consumer.descriptor)
	plan, err := Compile(CompileInput{
		Profile: profile, Bundles: []Bundle{provider, consumer}, Context: context.Background(),
		IncidentID: "incident-diagnostics", Recorder: recorder,
	})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := plan.Activate(context.Background(), "incident-diagnostics", recorder)
	if err != nil {
		t.Fatal(err)
	}
	product, err := activation.Root().OpenChild(context.Background(), ScopeProduct, ScopeOptions{ID: "product-desktop"})
	if err != nil {
		t.Fatal(err)
	}
	closed := 0
	if err := product.Own(NewEffect(func(context.Context) error { closed++; return nil })); err != nil {
		t.Fatal(err)
	}
	if err := activation.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("owned effect closes=%d", closed)
	}
	records := recorder.snapshot()
	for _, stage := range []DiagnosticStage{
		StageCompositionCompile, StageCompositionValidate, StageRouteCompile,
		StageBundleRegister, StageBundleStart, StageBundleReady, StageBundleStop,
		StageBundleDispose, StageScopeOpen, StageScopeClose,
	} {
		if !hasDiagnosticStage(records, stage) {
			t.Fatalf("missing stage %s in %#v", stage, records)
		}
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"diagnostic sink unavailable", "API_KEY", "Authorization", "private prompt"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("diagnostics leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestCOMP1ErrorsAndSnapshotsDoNotLeakOrRewriteActiveAttempts(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, key)
	consumer.fail = "start"
	privateFailure := errors.New("private prompt and API_KEY must remain private")
	consumer.failErr = privateFailure
	plan, err := Compile(CompileInput{Profile: testProfile(provider.descriptor, consumer.descriptor), Bundles: []Bundle{provider, consumer}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Activate(context.Background(), "incident-private-error", nil); err == nil ||
		!errors.Is(err, privateFailure) || strings.Contains(err.Error(), "private prompt") ||
		strings.Contains(err.Error(), "API_KEY") {
		t.Fatalf("unsafe activation error=%v", err)
	}

	consumer.fail = ""
	consumer.failErr = nil
	plan, err = Compile(CompileInput{Profile: testProfile(provider.descriptor, consumer.descriptor), Bundles: []Bundle{provider, consumer}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := plan.Snapshot()
	snapshot.Profile.Bundles[0].ID = "mutated"
	snapshot.Bundles[0].ID = "mutated"
	snapshot.Canonical[0] ^= 0xff
	if plan.Snapshot().Profile.Bundles[0].ID == "mutated" || plan.Snapshot().Bundles[0].ID == "mutated" ||
		bytes.Equal(snapshot.Canonical, plan.Snapshot().Canonical) {
		t.Fatal("snapshot clone mutated the plan")
	}
	activation, err := plan.Activate(context.Background(), "incident-frozen-attempt", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = activation.Close(context.Background()) }()
	product := mustChild(t, activation.Root(), ScopeProduct, "product-desktop", ScopeOptions{})
	conversation := mustChild(t, product, ScopeConversation, "conversation-frozen", ScopeOptions{})
	team := mustChild(t, conversation, ScopeTeam, "team-frozen", ScopeOptions{})
	agent := mustChild(t, team, ScopeAgent, "agent-frozen", ScopeOptions{})
	attempt := mustChild(t, agent, ScopeAttempt, "attempt-frozen", ScopeOptions{
		CompositionSnapshotDigest: plan.Snapshot().Digest, ExecutionBindingDigest: repeatHex("b"),
	})
	originalDigest := attempt.CompositionSnapshotDigest()
	profile := testProfile(provider.descriptor, consumer.descriptor)
	profile.FeatureFlags = []string{"later-snapshot"}
	later, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider, consumer}})
	if err != nil {
		t.Fatal(err)
	}
	if later.Snapshot().Digest == originalDigest || attempt.CompositionSnapshotDigest() != originalDigest {
		t.Fatal("later snapshot rewrote active attempt")
	}
}

func TestCOMP1ConcurrentCompileIsDeterministic(t *testing.T) {
	key := MustCapabilityKey[testPort](CapabilityObservabilityRecorder)
	provider, consumer := deterministicBundles(t, key)
	profile := testProfile(provider.descriptor, consumer.descriptor)
	want, err := Compile(CompileInput{Profile: profile, Bundles: []Bundle{provider, consumer}})
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	errorsFound := make(chan error, 32)
	for index := 0; index < 32; index++ {
		wait.Add(1)
		go func(reverse bool) {
			defer wait.Done()
			bundles := []Bundle{provider, consumer}
			if reverse {
				bundles = []Bundle{consumer, provider}
			}
			plan, err := Compile(CompileInput{Profile: profile, Bundles: bundles})
			if err != nil {
				errorsFound <- err
				return
			}
			if plan.Snapshot().Digest != want.Snapshot().Digest ||
				!bytes.Equal(plan.Snapshot().Canonical, want.Snapshot().Canonical) {
				errorsFound <- errors.New("nondeterministic compile")
			}
		}(index%2 == 0)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
}

type recordingDiagnostic struct {
	mu        sync.Mutex
	records   []DiagnosticRecord
	recordErr error
}

func (recorder *recordingDiagnostic) RecordCompositionDiagnostic(_ context.Context, record DiagnosticRecord) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.records = append(recorder.records, record)
	return recorder.recordErr
}

func (recorder *recordingDiagnostic) snapshot() []DiagnosticRecord {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]DiagnosticRecord(nil), recorder.records...)
}

func hasDiagnosticStage(records []DiagnosticRecord, stage DiagnosticStage) bool {
	for _, record := range records {
		if record.Stage == stage {
			return true
		}
	}
	return false
}

func deterministicBundles(
	t *testing.T,
	portKey CapabilityKey[testPort],
) (*testBundle, *testBundle) {
	t.Helper()
	events := []string{}
	provider := &testBundle{
		descriptor: BundleDescriptor{
			SchemaVersion: 1, ID: "loom-observability", Version: "1.0.0",
			LifecycleID: "observability-v1", Provides: []CapabilityRef{portKey.Ref()},
		},
		portKey: portKey, port: &testPortValue{name: "root-observability"}, events: &events,
	}
	consumer := &testBundle{
		descriptor: BundleDescriptor{
			SchemaVersion: 1, ID: "loom-agent-runtime", Version: "1.0.0",
			LifecycleID: "runtime-v1", Requires: []CapabilityRef{portKey.Ref()},
			Routes: []RouteDescriptor{{
				Method: "agent.dispatch", SchemaVersion: 1,
				OwnerBundle: "loom-agent-runtime", RequiredCapabilities: []CapabilityRef{portKey.Ref()},
				HandlerCapability: portKey.Ref(), PrivacyClass: PrivacyMetadataOnly,
			}},
		},
		requireKey: portKey, events: &events,
	}
	return provider, consumer
}

func testProfile(descriptors ...BundleDescriptor) LaunchProfile {
	bundles := make([]ProfileBundle, len(descriptors))
	for index, descriptor := range descriptors {
		bundles[index] = ProfileBundle{ID: descriptor.ID, Version: descriptor.Version, Mandatory: true}
	}
	return LaunchProfile{
		SchemaVersion: 1, ID: ProfileTest, Bundles: bundles,
		UnavailableRoutePolicy: RouteUnavailableFailStartup,
		ShutdownTimeoutMillis:  5_000,
		ProtectedCapabilities:  ProtectedCapabilities(),
	}
}

func withProvides(descriptor BundleDescriptor, refs ...CapabilityRef) BundleDescriptor {
	descriptor.Provides = append([]CapabilityRef(nil), refs...)
	return descriptor
}

func mustChild(
	t *testing.T,
	parent *CapabilityContext,
	kind ScopeKind,
	id string,
	options ScopeOptions,
) *CapabilityContext {
	t.Helper()
	options.ID = id
	child, err := parent.OpenChild(context.Background(), kind, options)
	if err != nil {
		t.Fatal(err)
	}
	return child
}

func repeatHex(character string) string {
	return string(bytes.Repeat([]byte(character), 64))
}
