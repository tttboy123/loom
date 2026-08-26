package credentials

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sort"

	"loom-pi-rebuild/internal/providerendpoint"
)

const (
	ImportProtocolOpenAIResponses  = "openai_responses"
	ImportModeExactProvider        = "exact_provider"
	ImportModeCustomEndpointReview = "custom_endpoint_review"
)

var ErrInvalidImportCandidate = errors.New("invalid credential import candidate")

// ImportCandidate is non-secret discovery metadata. It cannot authorize an
// import or execution; the selected source must lend its credential through
// UseAPIKey at the explicit mutation boundary.
type ImportCandidate struct {
	CandidateID         string   `json:"candidate_id"`
	CandidateDigest     string   `json:"candidate_digest"`
	SourceApplication   string   `json:"source_application"`
	DisplayName         string   `json:"display_name"`
	TargetProviderID    string   `json:"target_provider_id"`
	Protocol            string   `json:"protocol"`
	Endpoint            string   `json:"endpoint"`
	EndpointFingerprint string   `json:"endpoint_fingerprint"`
	ModelIDs            []string `json:"model_ids"`
	ImportMode          string   `json:"import_mode"`
	Current             bool     `json:"current"`
	CredentialAvailable bool     `json:"credential_available"`
	ReviewPolicyVersion uint64   `json:"review_policy_version"`
	ReviewPolicyDigest  string   `json:"review_policy_digest"`
}

// BindImportCandidate derives the immutable, non-secret endpoint review
// binding. Callers must re-bind a freshly discovered candidate before any
// approval or credential mutation so stale source metadata fails closed.
func BindImportCandidate(candidate ImportCandidate) (ImportCandidate, error) {
	if !validImportCandidateCore(candidate) {
		return ImportCandidate{}, ErrInvalidImportCandidate
	}
	policy, err := providerendpoint.NewEndpointPolicy(candidate.Endpoint)
	if err != nil {
		return ImportCandidate{}, ErrInvalidImportCandidate
	}
	models := append([]string(nil), candidate.ModelIDs...)
	sort.Strings(models)
	if hasDuplicateModel(models) {
		return ImportCandidate{}, ErrInvalidImportCandidate
	}

	candidate.Endpoint = policy.CanonicalEndpoint()
	candidate.EndpointFingerprint = policy.EndpointFingerprint()
	candidate.ReviewPolicyVersion = uint64(policy.Version())
	candidate.ReviewPolicyDigest = policy.Digest()
	candidate.CandidateDigest = importCandidateDigest(candidate, models)
	return candidate, nil
}

func importCandidateDigest(candidate ImportCandidate, sortedModels []string) string {
	hash := sha256.New()
	for _, value := range append([]string{
		"loom/credential-import-candidate/v1",
		candidate.CandidateID,
		candidate.SourceApplication,
		candidate.DisplayName,
		candidate.TargetProviderID,
		candidate.Protocol,
		candidate.Endpoint,
		candidate.EndpointFingerprint,
		candidate.ImportMode,
		candidate.ReviewPolicyDigest,
	}, sortedModels...) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	var version [8]byte
	binary.BigEndian.PutUint64(version[:], candidate.ReviewPolicyVersion)
	_, _ = hash.Write(version[:])
	return hex.EncodeToString(hash.Sum(nil))
}

// ImportCandidateModelDigest binds the model set independently for the
// endpoint approval authority and later frozen execution bindings.
func ImportCandidateModelDigest(candidate ImportCandidate) (string, error) {
	bound, err := BindImportCandidate(candidate)
	if err != nil {
		return "", err
	}
	models := append([]string(nil), bound.ModelIDs...)
	sort.Strings(models)
	hash := sha256.New()
	for _, value := range append([]string{
		"loom/credential-import-models/v1",
	}, models...) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validImportCandidateCore(candidate ImportCandidate) bool {
	if !validLowerHexDigest(candidate.CandidateID) || candidate.SourceApplication == "" ||
		candidate.DisplayName == "" || candidate.TargetProviderID == "" ||
		candidate.Protocol != ImportProtocolOpenAIResponses || len(candidate.ModelIDs) == 0 ||
		len(candidate.ModelIDs) > 64 || !candidate.CredentialAvailable {
		return false
	}
	return candidate.ImportMode == ImportModeExactProvider ||
		candidate.ImportMode == ImportModeCustomEndpointReview
}

func validLowerHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func hasDuplicateModel(models []string) bool {
	for index, model := range models {
		if model == "" || len(model) > 256 || index > 0 && model == models[index-1] {
			return true
		}
	}
	return false
}

type ImportSource interface {
	Discover(context.Context) ([]ImportCandidate, error)
	UseAPIKey(
		context.Context,
		string,
		func(context.Context, []byte) error,
	) error
}
