package verification

import (
	"errors"
	"strings"
	"testing"
)

func TestOutputContractClassifiesAllFourOutcomesDeterministically(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		emptyPolicy  EmptyOutputPolicy
		outputFrames int
		outputBytes  int
		result       bool
		terminal     string
		want         OutputClassification
	}{
		{
			name: "authorized non-empty", emptyPolicy: EmptyOutputInvalid,
			outputFrames: 1, outputBytes: 24, result: true,
			terminal: "succeeded", want: OutputValidNonEmpty,
		},
		{
			name: "contract-valid empty", emptyPolicy: EmptyOutputValid,
			result: true, terminal: "succeeded", want: OutputValidEmpty,
		},
		{
			name: "transient empty", emptyPolicy: EmptyOutputTransient,
			result: true, terminal: "succeeded", want: OutputTransientEmpty,
		},
		{
			name: "contract-invalid empty", emptyPolicy: EmptyOutputInvalid,
			result: true, terminal: "succeeded", want: OutputInvalid,
		},
		{
			name: "missing result", emptyPolicy: EmptyOutputValid,
			result: false, terminal: "succeeded", want: OutputInvalid,
		},
		{
			name: "failed runtime", emptyPolicy: EmptyOutputValid,
			result: true, terminal: "failed", want: OutputInvalid,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			contract, err := NewOutputContract(3, test.emptyPolicy)
			if err != nil {
				t.Fatal(err)
			}
			observation, err := NewOutputObservation(
				"team-evidence-1234567890abcdef1234567890abcdef",
				strings.Repeat("a", 64),
				strings.Repeat("b", 64),
				test.outputFrames+2,
				test.outputFrames,
				test.outputBytes,
				test.result,
				test.terminal,
			)
			if err != nil {
				t.Fatal(err)
			}
			first, err := Classify(contract, observation)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Classify(contract, observation)
			if err != nil {
				t.Fatal(err)
			}
			if first.Kind() != test.want ||
				first.Digest() == "" ||
				first.Digest() != second.Digest() ||
				first.OutputContractDigest() != contract.Digest() ||
				first.ObservationDigest() != observation.Digest() {
				t.Fatalf(
					"classification = %#v/%q second=%q contract=%q observation=%q",
					first,
					first.Digest(),
					second.Digest(),
					contract.Digest(),
					observation.Digest(),
				)
			}
		})
	}
}

func TestOutputObservationRejectsInconsistentOrUnboundedMetadata(t *testing.T) {
	t.Parallel()
	valid := func() OutputObservation {
		observation, err := NewOutputObservation(
			"team-evidence-1234567890abcdef1234567890abcdef",
			strings.Repeat("a", 64),
			strings.Repeat("b", 64),
			3,
			1,
			24,
			true,
			"succeeded",
		)
		if err != nil {
			t.Fatal(err)
		}
		return observation
	}
	if valid().Digest() == "" {
		t.Fatal("valid observation has empty digest")
	}
	tests := []struct {
		frames int
		output int
		bytes  int
	}{
		{frames: 0, output: 1, bytes: 1},
		{frames: 1, output: 2, bytes: 1},
		{frames: 2, output: 0, bytes: 1},
		{frames: 1025, output: 1, bytes: 1},
		{frames: 2, output: 1, bytes: (1 << 20) + 1},
	}
	for _, test := range tests {
		if _, err := NewOutputObservation(
			"team-evidence-1234567890abcdef1234567890abcdef",
			strings.Repeat("a", 64),
			strings.Repeat("b", 64),
			test.frames,
			test.output,
			test.bytes,
			true,
			"succeeded",
		); !errors.Is(err, ErrInvalidOutputObservation) {
			t.Fatalf("metadata %#v error = %v", test, err)
		}
	}
}

func TestOutputContractRejectsInvalidVersionAndPolicy(t *testing.T) {
	t.Parallel()
	if _, err := NewOutputContract(0, EmptyOutputValid); !errors.Is(
		err,
		ErrInvalidOutputContract,
	) {
		t.Fatalf("version error = %v", err)
	}
	if _, err := NewOutputContract(1, EmptyOutputPolicy("allow")); !errors.Is(
		err,
		ErrInvalidOutputContract,
	) {
		t.Fatalf("policy error = %v", err)
	}
}
