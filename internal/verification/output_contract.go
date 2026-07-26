package verification

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxOutputFrames = 1024
	maxOutputBytes  = 1 << 20
)

var (
	ErrInvalidOutputContract    = errors.New("invalid output contract")
	ErrInvalidOutputObservation = errors.New("invalid output observation")
)

type OutputClassification string

const (
	OutputValidNonEmpty  OutputClassification = "valid_nonempty"
	OutputValidEmpty     OutputClassification = "valid_empty"
	OutputTransientEmpty OutputClassification = "transient_empty"
	OutputInvalid        OutputClassification = "invalid"
)

type EmptyOutputPolicy string

const (
	EmptyOutputValid     EmptyOutputPolicy = "valid"
	EmptyOutputTransient EmptyOutputPolicy = "transient"
	EmptyOutputInvalid   EmptyOutputPolicy = "invalid"
)

type OutputContract struct {
	version int
	empty   EmptyOutputPolicy
	digest  string
}

type OutputObservation struct {
	evidenceID         string
	evidenceDigest     string
	summaryDigest      string
	authorizedFrames   int
	outputFrames       int
	outputPayloadBytes int
	resultObserved     bool
	terminalStatus     string
	digest             string
}

type Classification struct {
	kind                  OutputClassification
	outputContractVersion int
	outputContractDigest  string
	observationDigest     string
	evidenceID            string
	evidenceDigest        string
	summaryDigest         string
	terminalStatus        string
	digest                string
}

func NewOutputContract(
	version int,
	empty EmptyOutputPolicy,
) (OutputContract, error) {
	if version < 1 || version > 1_000_000 || !validEmptyPolicy(empty) {
		return OutputContract{}, ErrInvalidOutputContract
	}
	digest := canonicalOutputDigest(
		"loom.output-contract.v1",
		strconv.Itoa(version),
		string(empty),
	)
	return OutputContract{version: version, empty: empty, digest: digest}, nil
}

func NewOutputObservation(
	evidenceID string,
	evidenceDigest string,
	summaryDigest string,
	authorizedFrameCount int,
	outputFrameCount int,
	outputPayloadBytes int,
	resultObserved bool,
	terminalStatus string,
) (OutputObservation, error) {
	observation := OutputObservation{
		evidenceID:         evidenceID,
		evidenceDigest:     evidenceDigest,
		summaryDigest:      summaryDigest,
		authorizedFrames:   authorizedFrameCount,
		outputFrames:       outputFrameCount,
		outputPayloadBytes: outputPayloadBytes,
		resultObserved:     resultObserved,
		terminalStatus:     terminalStatus,
	}
	if !validOutputObservation(observation) {
		return OutputObservation{}, ErrInvalidOutputObservation
	}
	observation.digest = canonicalOutputDigest(
		"loom.output-observation.v1",
		observation.evidenceID,
		observation.evidenceDigest,
		observation.summaryDigest,
		strconv.Itoa(observation.authorizedFrames),
		strconv.Itoa(observation.outputFrames),
		strconv.Itoa(observation.outputPayloadBytes),
		strconv.FormatBool(observation.resultObserved),
		observation.terminalStatus,
	)
	return observation, nil
}

func Classify(
	contract OutputContract,
	observation OutputObservation,
) (Classification, error) {
	if !contract.Valid() {
		return Classification{}, ErrInvalidOutputContract
	}
	if !observation.Valid() {
		return Classification{}, ErrInvalidOutputObservation
	}
	kind := OutputInvalid
	if observation.resultObserved &&
		observation.terminalStatus == "succeeded" {
		if observation.outputFrames > 0 &&
			observation.outputPayloadBytes > 0 {
			kind = OutputValidNonEmpty
		} else {
			switch contract.empty {
			case EmptyOutputValid:
				kind = OutputValidEmpty
			case EmptyOutputTransient:
				kind = OutputTransientEmpty
			case EmptyOutputInvalid:
				kind = OutputInvalid
			}
		}
	}
	result := Classification{
		kind:                  kind,
		outputContractVersion: contract.version,
		outputContractDigest:  contract.digest,
		observationDigest:     observation.digest,
		evidenceID:            observation.evidenceID,
		evidenceDigest:        observation.evidenceDigest,
		summaryDigest:         observation.summaryDigest,
		terminalStatus:        observation.terminalStatus,
	}
	result.digest = canonicalOutputDigest(
		"loom.output-classification.v1",
		string(result.kind),
		strconv.Itoa(result.outputContractVersion),
		result.outputContractDigest,
		result.observationDigest,
		result.evidenceID,
		result.evidenceDigest,
		result.summaryDigest,
		result.terminalStatus,
	)
	return result, nil
}

func (contract OutputContract) Version() int                   { return contract.version }
func (contract OutputContract) EmptyPolicy() EmptyOutputPolicy { return contract.empty }
func (contract OutputContract) Digest() string                 { return contract.digest }
func (contract OutputContract) Valid() bool {
	if contract.version < 1 || !validEmptyPolicy(contract.empty) {
		return false
	}
	return contract.digest == canonicalOutputDigest(
		"loom.output-contract.v1",
		strconv.Itoa(contract.version),
		string(contract.empty),
	)
}

func (observation OutputObservation) EvidenceID() string {
	return observation.evidenceID
}
func (observation OutputObservation) EvidenceDigest() string {
	return observation.evidenceDigest
}
func (observation OutputObservation) SummaryDigest() string {
	return observation.summaryDigest
}
func (observation OutputObservation) AuthorizedFrameCount() int {
	return observation.authorizedFrames
}
func (observation OutputObservation) OutputFrameCount() int {
	return observation.outputFrames
}
func (observation OutputObservation) OutputPayloadBytes() int {
	return observation.outputPayloadBytes
}
func (observation OutputObservation) ResultObserved() bool {
	return observation.resultObserved
}
func (observation OutputObservation) TerminalStatus() string {
	return observation.terminalStatus
}
func (observation OutputObservation) Digest() string { return observation.digest }
func (observation OutputObservation) Valid() bool {
	if !validOutputObservation(observation) {
		return false
	}
	return observation.digest == canonicalOutputDigest(
		"loom.output-observation.v1",
		observation.evidenceID,
		observation.evidenceDigest,
		observation.summaryDigest,
		strconv.Itoa(observation.authorizedFrames),
		strconv.Itoa(observation.outputFrames),
		strconv.Itoa(observation.outputPayloadBytes),
		strconv.FormatBool(observation.resultObserved),
		observation.terminalStatus,
	)
}

func (classification Classification) Kind() OutputClassification {
	return classification.kind
}
func (classification Classification) OutputContractVersion() int {
	return classification.outputContractVersion
}
func (classification Classification) OutputContractDigest() string {
	return classification.outputContractDigest
}
func (classification Classification) ObservationDigest() string {
	return classification.observationDigest
}
func (classification Classification) EvidenceID() string {
	return classification.evidenceID
}
func (classification Classification) EvidenceDigest() string {
	return classification.evidenceDigest
}
func (classification Classification) SummaryDigest() string {
	return classification.summaryDigest
}
func (classification Classification) TerminalStatus() string {
	return classification.terminalStatus
}
func (classification Classification) Digest() string { return classification.digest }
func (classification Classification) Valid() bool {
	if !validOutputClassification(classification.kind) ||
		classification.outputContractVersion < 1 ||
		!validDigest(classification.outputContractDigest) ||
		!validDigest(classification.observationDigest) ||
		!validOutputID(classification.evidenceID) ||
		!validDigest(classification.evidenceDigest) ||
		!validDigest(classification.summaryDigest) ||
		!validTerminalStatus(classification.terminalStatus) {
		return false
	}
	return classification.digest == canonicalOutputDigest(
		"loom.output-classification.v1",
		string(classification.kind),
		strconv.Itoa(classification.outputContractVersion),
		classification.outputContractDigest,
		classification.observationDigest,
		classification.evidenceID,
		classification.evidenceDigest,
		classification.summaryDigest,
		classification.terminalStatus,
	)
}

func validOutputObservation(observation OutputObservation) bool {
	return validOutputID(observation.evidenceID) &&
		validDigest(observation.evidenceDigest) &&
		validDigest(observation.summaryDigest) &&
		observation.authorizedFrames >= 0 &&
		observation.authorizedFrames <= maxOutputFrames &&
		observation.outputFrames >= 0 &&
		observation.outputFrames <= observation.authorizedFrames &&
		observation.outputPayloadBytes >= 0 &&
		observation.outputPayloadBytes <= maxOutputBytes &&
		(observation.outputFrames == 0) ==
			(observation.outputPayloadBytes == 0) &&
		validTerminalStatus(observation.terminalStatus)
}

func validEmptyPolicy(policy EmptyOutputPolicy) bool {
	switch policy {
	case EmptyOutputValid, EmptyOutputTransient, EmptyOutputInvalid:
		return true
	default:
		return false
	}
}

func validOutputClassification(value OutputClassification) bool {
	switch value {
	case OutputValidNonEmpty, OutputValidEmpty, OutputTransientEmpty, OutputInvalid:
		return true
	default:
		return false
	}
}

func validTerminalStatus(value string) bool {
	switch value {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func validOutputID(value string) bool {
	if value == "" || len(value) > 128 ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if current < 0x21 || current == 0x7f {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func canonicalOutputDigest(schema string, fields ...string) string {
	hash := sha256.New()
	writeOutputDigestField(hash, schema)
	for _, field := range fields {
		writeOutputDigestField(hash, field)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type outputDigestWriter interface {
	Write([]byte) (int, error)
}

func writeOutputDigestField(writer outputDigestWriter, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(value))
}
