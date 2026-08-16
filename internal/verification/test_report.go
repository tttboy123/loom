package verification

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrUnsupportedTestCommand    = errors.New("unsupported governed test command")
	ErrInvalidGovernedTestReport = errors.New("invalid governed test report")
)

type TestRunner string

const (
	TestRunnerGo     TestRunner = "go_test"
	TestRunnerSwift  TestRunner = "swift_test"
	TestRunnerCargo  TestRunner = "cargo_test"
	TestRunnerPytest TestRunner = "pytest"
	TestRunnerNPM    TestRunner = "npm_test"
	TestRunnerPNPM   TestRunner = "pnpm_test"
	TestRunnerYarn   TestRunner = "yarn_test"
	TestRunnerBun    TestRunner = "bun_test"
)

type TestScope string

const (
	TestScopeDefault  TestScope = "default"
	TestScopeAll      TestScope = "all"
	TestScopeSelected TestScope = "selected"
)

type TestOutcome string

const (
	TestOutcomePassed TestOutcome = "passed"
	TestOutcomeFailed TestOutcome = "failed"
)

type GovernedTestCommand struct {
	runner        TestRunner
	scope         TestScope
	commandDigest string
}

type GovernedTestReportInput struct {
	CallID          string
	CallSequence    int64
	ArgumentsDigest string
	ExecutionID     string
	ExitCode        int
	OutputDigest    string
	DurationMS      int64
}

type GovernedTestReport struct {
	schemaVersion   int
	runner          TestRunner
	scope           TestScope
	outcome         TestOutcome
	commandDigest   string
	callID          string
	callSequence    int64
	argumentsDigest string
	executionID     string
	exitCode        int
	outputDigest    string
	durationMS      int64
	digest          string
}

type governedTestReportJSON struct {
	SchemaVersion   int         `json:"schema_version"`
	Runner          TestRunner  `json:"runner"`
	Scope           TestScope   `json:"scope"`
	Outcome         TestOutcome `json:"outcome"`
	CommandDigest   string      `json:"command_digest"`
	CallID          string      `json:"call_id"`
	CallSequence    int64       `json:"call_sequence"`
	ArgumentsDigest string      `json:"arguments_digest"`
	ExecutionID     string      `json:"execution_id"`
	ExitCode        int         `json:"exit_code"`
	OutputDigest    string      `json:"output_digest"`
	DurationMS      int64       `json:"duration_ms"`
	Digest          string      `json:"digest"`
}

func RecognizeGovernedTestCommand(command string) (GovernedTestCommand, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" || !strictTestCommandText(trimmed) {
		return GovernedTestCommand{}, ErrUnsupportedTestCommand
	}
	fields := strings.Fields(trimmed)
	runner, argumentStart, ok := governedTestRunner(fields)
	if !ok {
		return GovernedTestCommand{}, ErrUnsupportedTestCommand
	}
	scope := governedTestScope(runner, fields[argumentStart:])
	digestFields := append([]string{string(runner), string(scope)}, fields...)
	return GovernedTestCommand{
		runner: runner,
		scope:  scope,
		commandDigest: canonicalOutputDigest(
			"loom.governed-test-command.v1",
			digestFields...,
		),
	}, nil
}

func NewGovernedTestReport(
	command GovernedTestCommand,
	input GovernedTestReportInput,
) (GovernedTestReport, error) {
	if !command.Valid() || !validOutputID(input.CallID) ||
		input.CallSequence < 1 || input.CallSequence > 256 ||
		!validDigest(input.ArgumentsDigest) ||
		!validOutputID(input.ExecutionID) || input.ExitCode < 0 ||
		input.ExitCode > 255 || !validPrefixedTestDigest(input.OutputDigest) ||
		input.DurationMS < 0 || input.DurationMS > 3_600_000 {
		return GovernedTestReport{}, ErrInvalidGovernedTestReport
	}
	outcome := TestOutcomeFailed
	if input.ExitCode == 0 {
		outcome = TestOutcomePassed
	}
	report := GovernedTestReport{
		schemaVersion: 1,
		runner:        command.runner, scope: command.scope, outcome: outcome,
		commandDigest: command.commandDigest,
		callID:        input.CallID, callSequence: input.CallSequence,
		argumentsDigest: input.ArgumentsDigest, executionID: input.ExecutionID,
		exitCode: input.ExitCode, outputDigest: input.OutputDigest,
		durationMS: input.DurationMS,
	}
	report.digest = governedTestReportDigest(report)
	return report, nil
}

func DecodeGovernedTestReport(encoded []byte) (GovernedTestReport, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var transport governedTestReportJSON
	if decoder.Decode(&transport) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return GovernedTestReport{}, ErrInvalidGovernedTestReport
	}
	report := governedTestReportFromJSON(transport)
	if !report.Valid() {
		return GovernedTestReport{}, ErrInvalidGovernedTestReport
	}
	return report, nil
}

func (report GovernedTestReport) MarshalCanonicalJSON() ([]byte, error) {
	if !report.Valid() {
		return nil, ErrInvalidGovernedTestReport
	}
	return json.Marshal(governedTestReportToJSON(report))
}

func (command GovernedTestCommand) Runner() TestRunner    { return command.runner }
func (command GovernedTestCommand) Scope() TestScope      { return command.scope }
func (command GovernedTestCommand) CommandDigest() string { return command.commandDigest }
func (command GovernedTestCommand) Valid() bool {
	return validTestRunner(command.runner) && validTestScope(command.scope) &&
		validDigest(command.commandDigest)
}

func (report GovernedTestReport) SchemaVersion() int      { return report.schemaVersion }
func (report GovernedTestReport) Runner() TestRunner      { return report.runner }
func (report GovernedTestReport) Scope() TestScope        { return report.scope }
func (report GovernedTestReport) Outcome() TestOutcome    { return report.outcome }
func (report GovernedTestReport) CommandDigest() string   { return report.commandDigest }
func (report GovernedTestReport) CallID() string          { return report.callID }
func (report GovernedTestReport) CallSequence() int64     { return report.callSequence }
func (report GovernedTestReport) ArgumentsDigest() string { return report.argumentsDigest }
func (report GovernedTestReport) ExecutionID() string     { return report.executionID }
func (report GovernedTestReport) ExitCode() int           { return report.exitCode }
func (report GovernedTestReport) OutputDigest() string    { return report.outputDigest }
func (report GovernedTestReport) DurationMS() int64       { return report.durationMS }
func (report GovernedTestReport) Digest() string          { return report.digest }

func (report GovernedTestReport) Valid() bool {
	return report.schemaVersion == 1 && validTestRunner(report.runner) &&
		validTestScope(report.scope) && validTestOutcome(report.outcome) &&
		validDigest(report.commandDigest) && validOutputID(report.callID) &&
		report.callSequence >= 1 && report.callSequence <= 256 &&
		validDigest(report.argumentsDigest) && validOutputID(report.executionID) &&
		report.exitCode >= 0 && report.exitCode <= 255 &&
		validPrefixedTestDigest(report.outputDigest) &&
		report.durationMS >= 0 && report.durationMS <= 3_600_000 &&
		(report.exitCode == 0) == (report.outcome == TestOutcomePassed) &&
		report.digest == governedTestReportDigest(report)
}

func GovernedTestReportSetDigest(
	reports []GovernedTestReport,
) (string, error) {
	if len(reports) == 0 {
		return "", nil
	}
	if len(reports) > 256 {
		return "", ErrInvalidGovernedTestReport
	}
	ordered := append([]GovernedTestReport(nil), reports...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].CallSequence() < ordered[j].CallSequence()
	})
	fields := make([]string, 0, len(ordered)*2+1)
	fields = append(fields, strconv.Itoa(len(ordered)))
	for index, report := range ordered {
		if !report.Valid() || index > 0 &&
			report.CallSequence() == ordered[index-1].CallSequence() {
			return "", ErrInvalidGovernedTestReport
		}
		fields = append(
			fields,
			strconv.FormatInt(report.CallSequence(), 10),
			report.Digest(),
		)
	}
	return canonicalOutputDigest("loom.governed-test-report-set.v1", fields...), nil
}

func governedTestRunner(fields []string) (TestRunner, int, bool) {
	if len(fields) >= 2 {
		switch {
		case fields[0] == "go" && fields[1] == "test":
			return TestRunnerGo, 2, true
		case fields[0] == "swift" && fields[1] == "test":
			return TestRunnerSwift, 2, true
		case fields[0] == "cargo" && fields[1] == "test":
			return TestRunnerCargo, 2, true
		case (fields[0] == "python" || fields[0] == "python3") &&
			len(fields) >= 3 && fields[1] == "-m" && fields[2] == "pytest":
			return TestRunnerPytest, 3, true
		case fields[0] == "npm" && fields[1] == "test":
			return TestRunnerNPM, 2, true
		case fields[0] == "npm" && len(fields) >= 3 &&
			fields[1] == "run" && fields[2] == "test":
			return TestRunnerNPM, 3, true
		case fields[0] == "pnpm" && fields[1] == "test":
			return TestRunnerPNPM, 2, true
		case fields[0] == "yarn" && fields[1] == "test":
			return TestRunnerYarn, 2, true
		case fields[0] == "bun" && fields[1] == "test":
			return TestRunnerBun, 2, true
		}
	}
	if len(fields) >= 1 && fields[0] == "pytest" {
		return TestRunnerPytest, 1, true
	}
	return "", 0, false
}

func governedTestScope(runner TestRunner, arguments []string) TestScope {
	for _, argument := range arguments {
		switch argument {
		case "./...", "--workspace", "--workspaces", "--all", "--all-targets":
			return TestScopeAll
		}
	}
	if len(arguments) == 0 {
		return TestScopeDefault
	}
	if runner == TestRunnerNPM || runner == TestRunnerPNPM ||
		runner == TestRunnerYarn || runner == TestRunnerBun {
		return TestScopeSelected
	}
	for _, argument := range arguments {
		if !strings.HasPrefix(argument, "-") {
			return TestScopeSelected
		}
	}
	return TestScopeDefault
}

func strictTestCommandText(value string) bool {
	if len(value) > 4096 || strings.ContainsAny(value, "&|;<>`$()'\"\\\n\r\x00") {
		return false
	}
	for _, current := range value {
		if current < 0x20 && current != '\t' || current == 0x7f {
			return false
		}
	}
	return true
}

func validTestRunner(value TestRunner) bool {
	switch value {
	case TestRunnerGo, TestRunnerSwift, TestRunnerCargo, TestRunnerPytest,
		TestRunnerNPM, TestRunnerPNPM, TestRunnerYarn, TestRunnerBun:
		return true
	default:
		return false
	}
}

func validTestScope(value TestScope) bool {
	return value == TestScopeDefault || value == TestScopeAll ||
		value == TestScopeSelected
}

func validTestOutcome(value TestOutcome) bool {
	return value == TestOutcomePassed || value == TestOutcomeFailed
}

func validPrefixedTestDigest(value string) bool {
	return strings.HasPrefix(value, "sha256:") &&
		validDigest(strings.TrimPrefix(value, "sha256:"))
}

func governedTestReportDigest(report GovernedTestReport) string {
	return canonicalOutputDigest(
		"loom.governed-test-report.v1",
		strconv.Itoa(report.schemaVersion), string(report.runner),
		string(report.scope), string(report.outcome), report.commandDigest,
		report.callID, strconv.FormatInt(report.callSequence, 10),
		report.argumentsDigest, report.executionID,
		strconv.Itoa(report.exitCode), report.outputDigest,
		strconv.FormatInt(report.durationMS, 10),
	)
}

func governedTestReportToJSON(report GovernedTestReport) governedTestReportJSON {
	return governedTestReportJSON{
		SchemaVersion: report.schemaVersion, Runner: report.runner,
		Scope: report.scope, Outcome: report.outcome,
		CommandDigest: report.commandDigest, CallID: report.callID,
		CallSequence: report.callSequence, ArgumentsDigest: report.argumentsDigest,
		ExecutionID: report.executionID, ExitCode: report.exitCode,
		OutputDigest: report.outputDigest, DurationMS: report.durationMS,
		Digest: report.digest,
	}
}

func governedTestReportFromJSON(value governedTestReportJSON) GovernedTestReport {
	return GovernedTestReport{
		schemaVersion: value.SchemaVersion, runner: value.Runner,
		scope: value.Scope, outcome: value.Outcome,
		commandDigest: value.CommandDigest, callID: value.CallID,
		callSequence: value.CallSequence, argumentsDigest: value.ArgumentsDigest,
		executionID: value.ExecutionID, exitCode: value.ExitCode,
		outputDigest: value.OutputDigest, durationMS: value.DurationMS,
		digest: value.Digest,
	}
}
