package runtime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestPiRuntimeProbeConstructorRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	valid := validPiRuntimeProbeConfig()
	var typedNil *recordingPiMetadataRunner
	tests := []struct {
		name   string
		change func(*PiRuntimeProbeConfig)
	}{
		{name: "empty probe id", change: func(c *PiRuntimeProbeConfig) { c.ProbeID = "" }},
		{name: "empty instance id", change: func(c *PiRuntimeProbeConfig) { c.InstanceID = "" }},
		{name: "empty device id", change: func(c *PiRuntimeProbeConfig) { c.DeviceID = "" }},
		{name: "empty display name", change: func(c *PiRuntimeProbeConfig) { c.DisplayName = "" }},
		{name: "nil runner", change: func(c *PiRuntimeProbeConfig) { c.Runner = nil }},
		{name: "typed nil runner", change: func(c *PiRuntimeProbeConfig) { c.Runner = typedNil }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := valid
			test.change(&config)
			probe, err := NewPiRuntimeProbe(config)
			if !errors.Is(err, ErrInvalidPiRuntimeProbe) {
				t.Fatalf("NewPiRuntimeProbe() error = %v, want ErrInvalidPiRuntimeProbe", err)
			}
			if probe != nil {
				t.Fatalf("NewPiRuntimeProbe() probe = %#v, want nil", probe)
			}
		})
	}
}

func TestPiRuntimeProbeRequestsAndObservation(t *testing.T) {
	t.Parallel()

	runner := &recordingPiMetadataRunner{
		results: map[PiMetadataCommand]PiMetadataResult{
			PiMetadataVersion: {Stdout: "v0.73.1-rc.1+build\n"},
			PiMetadataListModels: {Stdout: strings.Join([]string{
				"provider model context max-out thinking images",
				"zeta model-z 200K 32K no yes",
				"alpha nested/model-a 1.5M 128K yes no",
			}, "\n")},
		},
	}
	config := validPiRuntimeProbeConfig()
	config.Runner = runner
	probe, err := NewPiRuntimeProbe(config)
	if err != nil {
		t.Fatalf("NewPiRuntimeProbe() error = %v", err)
	}
	if got := probe.ID(); got != config.ProbeID {
		t.Fatalf("ID() = %q, want %q", got, config.ProbeID)
	}

	observations, err := probe.ObserveRuntime(context.Background())
	if err != nil {
		t.Fatalf("ObserveRuntime() error = %v", err)
	}
	assertPiMetadataRequests(t, runner.calls)
	assertPiObservation(t, observations, RuntimeObservation{
		Instance: RuntimeInstance{
			ID:                   "runtime.pi.local",
			DeviceID:             "device.local",
			AdapterType:          "pi-cli",
			DisplayName:          "Local Pi",
			ExecutableVersion:    "v0.73.1-rc.1+build",
			Status:               RuntimeOnline,
			ObservedCapabilities: []string{"pi.metadata.models", "pi.metadata.version"},
			Capacity:             1,
		},
		ModelIDs: []string{"alpha/nested/model-a", "zeta/model-z"},
	})

	snapshot, err := DiscoverRuntime(context.Background(), []RuntimeProbe{probe})
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	got := snapshot.Observations()
	if len(got) != 1 || got[0].SourceProbeID != config.ProbeID {
		t.Fatalf("DiscoverRuntime() observations = %#v, want trusted source %q", got, config.ProbeID)
	}
	if snapshot.Digest() == "" {
		t.Fatal("DiscoverRuntime() returned empty digest")
	}
}

func TestPiRuntimeProbeNoModelsAndMutationIsolation(t *testing.T) {
	t.Parallel()

	runner := &recordingPiMetadataRunner{
		results: map[PiMetadataCommand]PiMetadataResult{
			PiMetadataVersion: {Stdout: "0.82.1"},
			PiMetadataListModels: {Stdout: strings.Join([]string{
				"No models available. Use /login to log into a provider via OAuth or API key. See:",
				"  /private/pi/docs/providers.md",
				"  /private/pi/docs/models.md",
			}, "\n")},
		},
		mutateArgs: true,
	}
	config := validPiRuntimeProbeConfig()
	config.Runner = runner
	probe, err := NewPiRuntimeProbe(config)
	if err != nil {
		t.Fatalf("NewPiRuntimeProbe() error = %v", err)
	}

	first, err := probe.ObserveRuntime(context.Background())
	if err != nil {
		t.Fatalf("first ObserveRuntime() error = %v", err)
	}
	assertPiMetadataRequests(t, runner.calls)
	if len(first) != 1 || first[0].ModelIDs != nil {
		t.Fatalf("first observations = %#v, want one observation with nil models", first)
	}
	first[0].Instance.ObservedCapabilities[0] = "mutated"
	first[0].ModelIDs = append(first[0].ModelIDs, "mutated/model")

	runner.calls = nil
	second, err := probe.ObserveRuntime(context.Background())
	if err != nil {
		t.Fatalf("second ObserveRuntime() error = %v", err)
	}
	assertPiMetadataRequests(t, runner.calls)
	if got := second[0].Instance.ObservedCapabilities; !reflect.DeepEqual(got, []string{"pi.metadata.models", "pi.metadata.version"}) {
		t.Fatalf("second capabilities = %#v, want immutable canonical capabilities", got)
	}
	if second[0].ModelIDs != nil {
		t.Fatalf("second models = %#v, want nil", second[0].ModelIDs)
	}
}

func TestPi0821NoModelsDiagnosticCompatibility(t *testing.T) {
	t.Parallel()

	headline := "No models available. Use /login to log into a provider via OAuth or API key. See:"
	providersPath := "/private/pi/docs/providers.md"
	modelsPath := "/private/pi/docs/models.md"
	valid := strings.Join([]string{
		headline,
		"  " + providersPath,
		"  " + modelsPath,
	}, "\n")
	if models, err := parsePiModels(valid); err != nil || models != nil {
		t.Fatalf("parsePiModels(valid) = (%#v, %v), want (nil, nil)", models, err)
	}
	if models, err := parsePiModels(valid + "\n"); err != nil || models != nil {
		t.Fatalf("parsePiModels(valid final LF) = (%#v, %v), want (nil, nil)", models, err)
	}
	for _, legacy := range []string{
		"No models available.",
		"No models available.\n",
	} {
		if models, err := parsePiModels(legacy); err != nil || models != nil {
			t.Fatalf("parsePiModels(legacy) = (%#v, %v), want (nil, nil)", models, err)
		}
	}

	invalid := []struct {
		name   string
		output string
	}{
		{name: "legacy headline with paths", output: "No models available.\n  " + providersPath + "\n  " + modelsPath},
		{name: "headline leading space", output: " " + headline + "\n  " + providersPath + "\n  " + modelsPath},
		{name: "headline trailing space", output: headline + " \n  " + providersPath + "\n  " + modelsPath},
		{name: "legacy diagnostic extra lines", output: "No models available.\n\n"},
		{name: "leading blank line", output: "\n" + valid},
		{name: "whole output leading space", output: " " + valid},
		{name: "providers zero indentation", output: strings.Replace(valid, "\n  "+providersPath, "\n"+providersPath, 1)},
		{name: "models zero indentation", output: strings.Replace(valid, "\n  "+modelsPath, "\n"+modelsPath, 1)},
		{name: "providers one space", output: strings.Replace(valid, "\n  "+providersPath, "\n "+providersPath, 1)},
		{name: "models one space", output: strings.Replace(valid, "\n  "+modelsPath, "\n "+modelsPath, 1)},
		{name: "providers three spaces", output: strings.Replace(valid, "\n  "+providersPath, "\n   "+providersPath, 1)},
		{name: "models three spaces", output: strings.Replace(valid, "\n  "+modelsPath, "\n   "+modelsPath, 1)},
		{name: "mixed one and two spaces", output: strings.Replace(valid, "\n  "+modelsPath, "\n "+modelsPath, 1)},
		{name: "mixed two and three spaces", output: strings.Replace(valid, "\n  "+modelsPath, "\n   "+modelsPath, 1)},
		{name: "tab indentation", output: strings.Replace(valid, "\n  "+providersPath, "\n\t"+providersPath, 1)},
		{name: "vertical tab indentation", output: strings.Replace(valid, "\n  "+providersPath, "\n\v"+providersPath, 1)},
		{name: "non ascii indentation", output: strings.Replace(valid, "\n  "+providersPath, "\n\u00a0"+providersPath, 1)},
		{name: "two spaces then tab", output: strings.Replace(valid, "\n  "+providersPath, "\n  \t"+providersPath, 1)},
		{name: "blank line before providers", output: strings.Replace(valid, "\n  "+providersPath, "\n\n  "+providersPath, 1)},
		{name: "whitespace line before providers", output: strings.Replace(valid, "\n  "+providersPath, "\n \n  "+providersPath, 1)},
		{name: "providers trailing space", output: strings.Replace(valid, providersPath, providersPath+" ", 1)},
		{name: "models trailing space", output: strings.Replace(valid, modelsPath, modelsPath+" ", 1)},
		{name: "extra final LF", output: valid + "\n\n"},
		{name: "extra whitespace line", output: valid + "\n \n"},
		{name: "extra path", output: valid + "\n  /private/pi/docs/extra.md"},
		{name: "relative models path", output: strings.Replace(valid, modelsPath, "relative/docs/models.md", 1)},
		{name: "different parent", output: strings.Replace(valid, modelsPath, "/other/pi/docs/models.md", 1)},
		{name: "non docs parents", output: strings.ReplaceAll(valid, "/private/pi/docs/", "/private/pi/not-docs/")},
		{name: "providers basename swapped", output: strings.Replace(valid, providersPath, modelsPath, 1)},
		{name: "unclean providers path", output: strings.Replace(valid, providersPath, "/private/pi/../pi/docs/providers.md", 1)},
		{name: "providers embedded control", output: strings.Replace(valid, providersPath, "/private/pi/docs/provid\x1fers.md", 1)},
		{name: "providers trailing tab", output: strings.Replace(valid, providersPath, providersPath+"\t", 1)},
		{name: "models trailing vertical tab", output: strings.Replace(valid, modelsPath, modelsPath+"\v", 1)},
		{name: "carriage return", output: valid + "\r\n"},
	}
	for _, test := range invalid {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if models, err := parsePiModels(test.output); !errors.Is(err, ErrInvalidPiMetadataOutput) || models != nil {
				t.Fatalf("parsePiModels() = (%#v, %v), want ErrInvalidPiMetadataOutput", models, err)
			}
		})
	}
}

func TestPiRuntimeProbeCancellationAndRunnerFailureReturnNoPartialObservation(t *testing.T) {
	t.Parallel()

	t.Run("pre-canceled", func(t *testing.T) {
		runner := successfulPiMetadataRunner()
		config := validPiRuntimeProbeConfig()
		config.Runner = runner
		probe := mustPiRuntimeProbe(t, config)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, err := probe.ObserveRuntime(ctx)
		if !errors.Is(err, context.Canceled) || len(got) != 0 {
			t.Fatalf("ObserveRuntime() = (%#v, %v), want no observation and context.Canceled", got, err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("calls = %#v, want none", runner.calls)
		}
	})

	t.Run("version failure skips models", func(t *testing.T) {
		runner := successfulPiMetadataRunner()
		runner.errs = map[PiMetadataCommand]error{PiMetadataVersion: errors.New("secret-version-runner")}
		probe := mustPiRuntimeProbeWithRunner(t, runner)
		got, err := probe.ObserveRuntime(context.Background())
		assertPiCommandFailure(t, got, err, "secret-version-runner")
		if len(runner.calls) != 1 || runner.calls[0].Command != PiMetadataVersion {
			t.Fatalf("calls = %#v, want version only", runner.calls)
		}
	})

	t.Run("model failure returns no version-only partial", func(t *testing.T) {
		runner := successfulPiMetadataRunner()
		runner.errs = map[PiMetadataCommand]error{PiMetadataListModels: errors.New("secret-model-runner")}
		probe := mustPiRuntimeProbeWithRunner(t, runner)
		got, err := probe.ObserveRuntime(context.Background())
		assertPiCommandFailure(t, got, err, "secret-model-runner")
		assertPiMetadataRequests(t, runner.calls)
	})

	t.Run("canceled between commands", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		runner := successfulPiMetadataRunner()
		runner.afterCall = func(command PiMetadataCommand) {
			if command == PiMetadataVersion {
				cancel()
			}
		}
		probe := mustPiRuntimeProbeWithRunner(t, runner)
		got, err := probe.ObserveRuntime(ctx)
		if !errors.Is(err, context.Canceled) || len(got) != 0 {
			t.Fatalf("ObserveRuntime() = (%#v, %v), want no observation and context.Canceled", got, err)
		}
		if len(runner.calls) != 1 {
			t.Fatalf("calls = %#v, want version only", runner.calls)
		}
	})

	t.Run("canceled after models", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		runner := successfulPiMetadataRunner()
		runner.afterCall = func(command PiMetadataCommand) {
			if command == PiMetadataListModels {
				cancel()
			}
		}
		probe := mustPiRuntimeProbeWithRunner(t, runner)
		got, err := probe.ObserveRuntime(ctx)
		if !errors.Is(err, context.Canceled) || len(got) != 0 {
			t.Fatalf("ObserveRuntime() = (%#v, %v), want no observation and context.Canceled", got, err)
		}
		assertPiMetadataRequests(t, runner.calls)
	})
}

func TestParsePiMetadataOutputBoundary(t *testing.T) {
	t.Parallel()

	largeOutput := strings.Repeat("a", 256*1024+1)
	manyRows := []string{"provider model context max-out thinking images"}
	for index := 0; index < 1025; index++ {
		manyRows = append(manyRows, fmt.Sprintf("provider model-%04d 1K 1K no no", index))
	}

	tests := []struct {
		name         string
		version      PiMetadataResult
		models       PiMetadataResult
		want         error
		secretMarker string
	}{
		{name: "version stdout too large", version: PiMetadataResult{Stdout: largeOutput}, models: validPiModelsResult(), want: ErrPiMetadataOutputTooLarge},
		{name: "model stderr too large", version: validPiVersionResult(), models: PiMetadataResult{Stderr: largeOutput}, want: ErrPiMetadataOutputTooLarge},
		{name: "invalid version utf8", version: PiMetadataResult{Stdout: string([]byte{0xff})}, models: validPiModelsResult(), want: ErrInvalidPiMetadataOutput},
		{name: "invalid model utf8", version: validPiVersionResult(), models: PiMetadataResult{Stdout: string([]byte{0xff})}, want: ErrInvalidPiMetadataOutput},
		{name: "version nul", version: PiMetadataResult{Stdout: "0.73\x001"}, models: validPiModelsResult(), want: ErrInvalidPiMetadataOutput},
		{name: "model nul", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np m 1K 1K no no\x00"}, want: ErrInvalidPiMetadataOutput},
		{name: "version stderr", version: PiMetadataResult{Stdout: "0.73.1", Stderr: "secret-version-stderr"}, models: validPiModelsResult(), want: ErrPiMetadataStderr, secretMarker: "secret-version-stderr"},
		{name: "model stderr", version: validPiVersionResult(), models: PiMetadataResult{Stdout: validPiModelsResult().Stdout, Stderr: "secret-model-stderr"}, want: ErrPiMetadataStderr, secretMarker: "secret-model-stderr"},
		{name: "empty version", version: PiMetadataResult{}, models: validPiModelsResult(), want: ErrInvalidPiMetadataOutput},
		{name: "multiline version", version: PiMetadataResult{Stdout: "0.73.1\nsecret-output"}, models: validPiModelsResult(), want: ErrInvalidPiMetadataOutput, secretMarker: "secret-output"},
		{name: "version whitespace", version: PiMetadataResult{Stdout: "version 0.73.1"}, models: validPiModelsResult(), want: ErrInvalidPiMetadataOutput},
		{name: "version punctuation", version: PiMetadataResult{Stdout: "0.73.1/unsafe"}, models: validPiModelsResult(), want: ErrInvalidPiMetadataOutput},
		{name: "version over 128", version: PiMetadataResult{Stdout: strings.Repeat("v", 129)}, models: validPiModelsResult(), want: ErrInvalidPiMetadataOutput},
		{name: "wrong header", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context maxout thinking images"}, want: ErrInvalidPiMetadataOutput},
		{name: "wrong column count", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np m 1K 1K no"}, want: ErrInvalidPiMetadataOutput},
		{name: "provider slash", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np/x m 1K 1K no no"}, want: ErrInvalidPiMetadataOutput},
		{name: "provider over 256", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\n" + strings.Repeat("p", 257) + " m 1K 1K no no"}, want: ErrInvalidPiMetadataOutput},
		{name: "model over 256", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np " + strings.Repeat("m", 257) + " 1K 1K no no"}, want: ErrInvalidPiMetadataOutput},
		{name: "invalid context", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np m 0 1K no no"}, want: ErrInvalidPiMetadataOutput},
		{name: "invalid max out", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np m 1K 1.2.3K no no"}, want: ErrInvalidPiMetadataOutput},
		{name: "invalid thinking", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np m 1K 1K true no"}, want: ErrInvalidPiMetadataOutput},
		{name: "invalid images", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np m 1K 1K no false"}, want: ErrInvalidPiMetadataOutput},
		{name: "duplicate model", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\np m 1K 1K no no\np m 2K 2K yes yes"}, want: ErrDuplicatePiRuntimeModel},
		{name: "prose in table", version: validPiVersionResult(), models: PiMetadataResult{Stdout: "provider model context max-out thinking images\nsecret prose mixed into output"}, want: ErrInvalidPiMetadataOutput, secretMarker: "secret prose"},
		{name: "row ceiling", version: validPiVersionResult(), models: PiMetadataResult{Stdout: strings.Join(manyRows, "\n")}, want: ErrInvalidPiMetadataOutput},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runner := &recordingPiMetadataRunner{
				results: map[PiMetadataCommand]PiMetadataResult{
					PiMetadataVersion:    test.version,
					PiMetadataListModels: test.models,
				},
			}
			probe := mustPiRuntimeProbeWithRunner(t, runner)
			got, err := probe.ObserveRuntime(context.Background())
			if !errors.Is(err, test.want) {
				t.Fatalf("ObserveRuntime() error = %v, want %v", err, test.want)
			}
			if len(got) != 0 {
				t.Fatalf("ObserveRuntime() observations = %#v, want none", got)
			}
			if test.secretMarker != "" && strings.Contains(err.Error(), test.secretMarker) {
				t.Fatalf("error leaked raw metadata %q: %v", test.secretMarker, err)
			}
		})
	}
}

func validPiRuntimeProbeConfig() PiRuntimeProbeConfig {
	return PiRuntimeProbeConfig{
		ProbeID:     "probe.pi.local",
		InstanceID:  "runtime.pi.local",
		DeviceID:    "device.local",
		DisplayName: "Local Pi",
		Runner:      successfulPiMetadataRunner(),
	}
}

func successfulPiMetadataRunner() *recordingPiMetadataRunner {
	return &recordingPiMetadataRunner{
		results: map[PiMetadataCommand]PiMetadataResult{
			PiMetadataVersion:    validPiVersionResult(),
			PiMetadataListModels: validPiModelsResult(),
		},
	}
}

func validPiVersionResult() PiMetadataResult {
	return PiMetadataResult{Stdout: "0.73.1\n"}
}

func validPiModelsResult() PiMetadataResult {
	return PiMetadataResult{Stdout: "provider model context max-out thinking images\nprovider model 200K 32K no no\n"}
}

func mustPiRuntimeProbeWithRunner(t *testing.T, runner PiMetadataRunner) RuntimeProbe {
	t.Helper()
	config := validPiRuntimeProbeConfig()
	config.Runner = runner
	return mustPiRuntimeProbe(t, config)
}

func mustPiRuntimeProbe(t *testing.T, config PiRuntimeProbeConfig) RuntimeProbe {
	t.Helper()
	probe, err := NewPiRuntimeProbe(config)
	if err != nil {
		t.Fatalf("NewPiRuntimeProbe() error = %v", err)
	}
	return probe
}

func assertPiCommandFailure(t *testing.T, got []RuntimeObservation, err error, secret string) {
	t.Helper()
	if !errors.Is(err, ErrPiMetadataCommandFailed) {
		t.Fatalf("ObserveRuntime() error = %v, want ErrPiMetadataCommandFailed", err)
	}
	if len(got) != 0 {
		t.Fatalf("ObserveRuntime() observations = %#v, want none", got)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("ObserveRuntime() error leaked runner text %q: %v", secret, err)
	}
}

func assertPiMetadataRequests(t *testing.T, calls []PiMetadataRequest) {
	t.Helper()
	want := []PiMetadataRequest{
		{Command: PiMetadataVersion, Args: []string{"--version"}},
		{Command: PiMetadataListModels, Args: []string{
			"--offline",
			"--no-approve",
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-themes",
			"--no-context-files",
			"--list-models",
		}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("metadata calls = %#v, want %#v", calls, want)
	}
}

func assertPiObservation(t *testing.T, got []RuntimeObservation, want RuntimeObservation) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("observations = %#v, want exactly one", got)
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Fatalf("observation = %#v, want %#v", got[0], want)
	}
}

type recordingPiMetadataRunner struct {
	results    map[PiMetadataCommand]PiMetadataResult
	errs       map[PiMetadataCommand]error
	calls      []PiMetadataRequest
	mutateArgs bool
	afterCall  func(PiMetadataCommand)
}

func (r *recordingPiMetadataRunner) RunPiMetadata(_ context.Context, request PiMetadataRequest) (PiMetadataResult, error) {
	copied := PiMetadataRequest{
		Command: request.Command,
		Args:    append([]string(nil), request.Args...),
	}
	r.calls = append(r.calls, copied)
	if r.mutateArgs && len(request.Args) > 0 {
		request.Args[0] = "--mutated"
	}
	if r.afterCall != nil {
		r.afterCall(request.Command)
	}
	if err := r.errs[request.Command]; err != nil {
		return PiMetadataResult{}, err
	}
	return r.results[request.Command], nil
}
