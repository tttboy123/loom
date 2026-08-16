package verification

import (
	"errors"
	"strings"
	"testing"
)

func TestRecognizeGovernedTestCommandUsesStrictSingleCommandGrammar(t *testing.T) {
	tests := []struct {
		command string
		runner  TestRunner
		scope   TestScope
	}{
		{"go test", TestRunnerGo, TestScopeDefault},
		{"go test ./... -count=1", TestRunnerGo, TestScopeAll},
		{"go test ./internal/app", TestRunnerGo, TestScopeSelected},
		{"swift test", TestRunnerSwift, TestScopeDefault},
		{"cargo test --workspace", TestRunnerCargo, TestScopeAll},
		{"pytest tests/unit", TestRunnerPytest, TestScopeSelected},
		{"python -m pytest", TestRunnerPytest, TestScopeDefault},
		{"python3 -m pytest tests", TestRunnerPytest, TestScopeSelected},
		{"npm test", TestRunnerNPM, TestScopeDefault},
		{"npm run test -- --runInBand", TestRunnerNPM, TestScopeSelected},
		{"pnpm test", TestRunnerPNPM, TestScopeDefault},
		{"yarn test", TestRunnerYarn, TestScopeDefault},
		{"bun test", TestRunnerBun, TestScopeDefault},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			classification, err := RecognizeGovernedTestCommand(test.command)
			if err != nil {
				t.Fatal(err)
			}
			if classification.Runner() != test.runner ||
				classification.Scope() != test.scope ||
				!classification.Valid() ||
				!validTestDigest(classification.CommandDigest()) {
				t.Fatalf("classification = %#v", classification)
			}
		})
	}

	for _, command := range []string{
		"", "go build ./...", "GOFLAGS=-count=1 go test ./...",
		"go test ./... && curl https://example.com",
		"go test ./... | tee result.txt", "go test ./... > result.txt",
		"go test $(cat scope)", "go test `cat scope`", "go test\n./...",
		"go test './...'", "sh -c go-test",
	} {
		if _, err := RecognizeGovernedTestCommand(command); !errors.Is(
			err,
			ErrUnsupportedTestCommand,
		) {
			t.Fatalf("RecognizeGovernedTestCommand(%q) error = %v", command, err)
		}
	}
}

func TestGovernedTestReportBindsExactToolResultWithoutCommandContent(t *testing.T) {
	classification, err := RecognizeGovernedTestCommand("go test ./... -count=1")
	if err != nil {
		t.Fatal(err)
	}
	input := GovernedTestReportInput{
		CallID:          "call-test-1",
		CallSequence:    2,
		ArgumentsDigest: strings.Repeat("a", 64),
		ExecutionID:     "execution-test-1",
		ExitCode:        0,
		OutputDigest:    "sha256:" + strings.Repeat("b", 64),
		DurationMS:      1250,
	}
	report, err := NewGovernedTestReport(classification, input)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid() || report.SchemaVersion() != 1 ||
		report.Runner() != TestRunnerGo || report.Scope() != TestScopeAll ||
		report.Outcome() != TestOutcomePassed || report.ExitCode() != 0 ||
		report.CommandDigest() != classification.CommandDigest() ||
		report.ArgumentsDigest() != input.ArgumentsDigest ||
		report.OutputDigest() != input.OutputDigest ||
		!validTestDigest(report.Digest()) {
		t.Fatalf("report = %#v", report)
	}
	encoded, err := report.MarshalCanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "go test") ||
		strings.Contains(string(encoded), "./...") {
		t.Fatalf("command content entered report JSON: %s", encoded)
	}
	reopened, err := DecodeGovernedTestReport(encoded)
	if err != nil || reopened.Digest() != report.Digest() || !reopened.Valid() {
		t.Fatalf("reopened=%#v error=%v", reopened, err)
	}

	failedInput := input
	failedInput.ExecutionID = "execution-test-2"
	failedInput.ExitCode = 1
	failed, err := NewGovernedTestReport(classification, failedInput)
	if err != nil || failed.Outcome() != TestOutcomeFailed || !failed.Valid() {
		t.Fatalf("failed report=%#v error=%v", failed, err)
	}
}

func TestGovernedTestReportRejectsInvalidAndSubstitutedFields(t *testing.T) {
	classification, err := RecognizeGovernedTestCommand("go test ./...")
	if err != nil {
		t.Fatal(err)
	}
	valid := GovernedTestReportInput{
		CallID: "call-test-1", CallSequence: 1,
		ArgumentsDigest: strings.Repeat("a", 64),
		ExecutionID:     "execution-test-1", ExitCode: 0,
		OutputDigest: "sha256:" + strings.Repeat("b", 64), DurationMS: 1,
	}
	mutations := []func(*GovernedTestReportInput){
		func(value *GovernedTestReportInput) { value.CallID = "" },
		func(value *GovernedTestReportInput) { value.CallSequence = 0 },
		func(value *GovernedTestReportInput) { value.ArgumentsDigest = strings.Repeat("a", 63) },
		func(value *GovernedTestReportInput) { value.ExecutionID = "" },
		func(value *GovernedTestReportInput) { value.ExitCode = -1 },
		func(value *GovernedTestReportInput) { value.ExitCode = 256 },
		func(value *GovernedTestReportInput) { value.OutputDigest = strings.Repeat("b", 64) },
		func(value *GovernedTestReportInput) { value.DurationMS = -1 },
	}
	for index, mutate := range mutations {
		candidate := valid
		mutate(&candidate)
		if _, err := NewGovernedTestReport(classification, candidate); !errors.Is(
			err,
			ErrInvalidGovernedTestReport,
		) {
			t.Fatalf("mutation %d error = %v", index, err)
		}
	}

	report, err := NewGovernedTestReport(classification, valid)
	if err != nil {
		t.Fatal(err)
	}
	report.argumentsDigest = strings.Repeat("c", 64)
	if report.Valid() {
		t.Fatal("arguments substitution remained valid")
	}
}

func TestGovernedTestReportSetDigestIsOrderIndependentAndRejectsSequenceReuse(t *testing.T) {
	command, err := RecognizeGovernedTestCommand("go test ./...")
	if err != nil {
		t.Fatal(err)
	}
	makeReport := func(callID string, sequence int64) GovernedTestReport {
		report, reportErr := NewGovernedTestReport(command, GovernedTestReportInput{
			CallID: callID, CallSequence: sequence,
			ArgumentsDigest: strings.Repeat(string(rune('a'+sequence)), 64),
			ExecutionID:     "execution-" + callID, ExitCode: 0,
			OutputDigest: "sha256:" + strings.Repeat(string(rune('d'+sequence)), 64),
			DurationMS:   sequence,
		})
		if reportErr != nil {
			t.Fatal(reportErr)
		}
		return report
	}
	first, second := makeReport("call-a", 1), makeReport("call-b", 2)
	left, err := GovernedTestReportSetDigest([]GovernedTestReport{first, second})
	if err != nil {
		t.Fatal(err)
	}
	right, err := GovernedTestReportSetDigest([]GovernedTestReport{second, first})
	if err != nil || left != right || !validTestDigest(left) {
		t.Fatalf("set digests left=%q right=%q error=%v", left, right, err)
	}
	if _, err := GovernedTestReportSetDigest(
		[]GovernedTestReport{first, first},
	); !errors.Is(err, ErrInvalidGovernedTestReport) {
		t.Fatalf("duplicate sequence error = %v", err)
	}
}

func validTestDigest(value string) bool {
	return len(value) == 64
}
