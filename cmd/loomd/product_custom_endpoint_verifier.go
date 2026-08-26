package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/providerendpoint"
)

const customEndpointVerificationResponseLimit = 64 << 10

var errProductCustomEndpointVerification = errors.New("custom endpoint verification unavailable")

type productAccountCredentialVerifier struct {
	fixed   credentials.CredentialVerifier
	journal *journal.Store
}

func newProductAccountCredentialVerifier(
	fixed credentials.CredentialVerifier,
	journalStore *journal.Store,
) (*productAccountCredentialVerifier, error) {
	if fixed == nil || journalStore == nil {
		return nil, errProductCustomEndpointVerification
	}
	return &productAccountCredentialVerifier{fixed: fixed, journal: journalStore}, nil
}

func (verifier *productAccountCredentialVerifier) Verify(
	ctx context.Context,
	providerID string,
	secret []byte,
) (credentials.VerificationResult, error) {
	if verifier == nil || verifier.fixed == nil {
		return credentials.VerificationResult{}, errProductCustomEndpointVerification
	}
	return verifier.fixed.Verify(ctx, providerID, secret)
}

func (verifier *productAccountCredentialVerifier) VerifyAccount(
	ctx context.Context,
	binding credentials.CredentialVerificationBinding,
	secret []byte,
) (credentials.VerificationResult, error) {
	if verifier == nil || ctx == nil || binding.ProviderID == "" ||
		binding.ProviderAccountID == "" || binding.CredentialReference == "" ||
		binding.CredentialRevision <= 0 || len(secret) == 0 {
		return credentials.VerificationResult{}, errProductCustomEndpointVerification
	}
	if binding.ProviderID != "custom-openai" {
		return verifier.fixed.Verify(ctx, binding.ProviderID, secret)
	}
	review, err := verifier.approvedReview(ctx, binding.ProviderID, binding.ProviderAccountID)
	if err != nil {
		return unavailableCustomEndpointVerification(), nil
	}
	policy, err := providerendpoint.NewEndpointPolicy(review.Endpoint)
	if err != nil || policy.EndpointFingerprint() != review.EndpointFingerprint ||
		uint64(policy.Version()) != review.ReviewPolicyVersion ||
		policy.Digest() != review.ReviewPolicyDigest {
		return unavailableCustomEndpointVerification(), nil
	}
	client, err := providerendpoint.NewHTTPClient(providerendpoint.HTTPConfig{
		Policy: policy, Timeout: 5 * time.Second,
	})
	if err != nil {
		return unavailableCustomEndpointVerification(), nil
	}
	modelsURL, err := customEndpointModelsURL(policy.CanonicalEndpoint())
	if err != nil {
		return unavailableCustomEndpointVerification(), nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return unavailableCustomEndpointVerification(), nil
	}
	request.Header.Set("Authorization", "Bearer "+string(secret))
	request.Header.Set("Accept", "application/json")
	response, callErr := client.Do(request)
	request.Header.Del("Authorization")
	if callErr != nil || response == nil || response.Body == nil {
		return unavailableCustomEndpointVerification(), nil
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(
		response.Body, customEndpointVerificationResponseLimit+1,
	))
	if readErr != nil || len(body) > customEndpointVerificationResponseLimit {
		return unavailableCustomEndpointVerification(), nil
	}
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300 && json.Valid(body):
		return credentials.VerificationResult{
			Status: credentials.VerificationValid, SafeMessage: "credential verified",
		}, nil
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		return credentials.VerificationResult{
			Status:      credentials.VerificationRejected,
			Reason:      credentials.VerificationReasonProviderRejected,
			SafeMessage: "credential rejected",
		}, nil
	default:
		return unavailableCustomEndpointVerification(), nil
	}
}

func (verifier *productAccountCredentialVerifier) approvedReview(
	ctx context.Context,
	providerID, accountID string,
) (app.EndpointReviewResult, error) {
	events, err := verifier.journal.ReadAll(ctx)
	if err != nil {
		return app.EndpointReviewResult{}, errProductCustomEndpointVerification
	}
	now := time.Now().UTC()
	superseded := make(map[string]app.EndpointReviewResult)
	for _, event := range events {
		if event.Type != "ProviderEndpointReviewSuperseded" {
			continue
		}
		var review app.EndpointReviewResult
		if json.Unmarshal(event.PayloadJSON, &review) != nil ||
			!validProductEndpointReview(review) {
			return app.EndpointReviewResult{}, errProductCustomEndpointVerification
		}
		if _, duplicate := superseded[review.ApprovalDigest]; duplicate {
			return app.EndpointReviewResult{}, errProductCustomEndpointVerification
		}
		superseded[review.ApprovalDigest] = review
	}
	var selected app.EndpointReviewResult
	found := false
	for _, event := range events {
		if event.Type != "ProviderEndpointReviewApproved" {
			continue
		}
		var candidate app.EndpointReviewResult
		if json.Unmarshal(event.PayloadJSON, &candidate) != nil ||
			candidate.ProviderID != providerID || candidate.ProviderAccountID != accountID {
			continue
		}
		if !validProductEndpointReview(candidate) {
			return app.EndpointReviewResult{}, errProductCustomEndpointVerification
		}
		if superseding, isSuperseded := superseded[candidate.ApprovalDigest]; isSuperseded {
			if superseding != candidate {
				return app.EndpointReviewResult{}, errProductCustomEndpointVerification
			}
			continue
		}
		expiresAt, parseErr := time.Parse(time.RFC3339Nano, candidate.ExpiresAt)
		if parseErr != nil || !now.Before(expiresAt) {
			continue
		}
		if found {
			return app.EndpointReviewResult{}, errProductCustomEndpointVerification
		}
		selected, found = candidate, true
	}
	if !found {
		return app.EndpointReviewResult{}, errProductCustomEndpointVerification
	}
	return selected, nil
}

func validProductEndpointReview(review app.EndpointReviewResult) bool {
	approvedAt, approvedErr := time.Parse(time.RFC3339Nano, review.ApprovedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, review.ExpiresAt)
	return validProductHex(review.CandidateID, 64) &&
		validProductHex(review.CandidateDigest, 64) &&
		validProductHex(review.AuthorityCandidateDigest, 64) &&
		credentials.ValidProviderAccountIdentifier(
			review.ProviderID, review.ProviderAccountID,
		) && review.Protocol != "" && review.Endpoint != "" &&
		validProductHex(review.EndpointFingerprint, 64) &&
		validProductHex(review.ModelDigest, 64) && review.ReviewPolicyVersion > 0 &&
		validProductHex(review.ReviewPolicyDigest, 64) &&
		review.Status == "approved" && review.Revision == 2 &&
		validProductHex(review.ApprovalDigest, 64) && approvedErr == nil &&
		expiresErr == nil && approvedAt.Before(expiresAt)
}

func customEndpointModelsURL(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errProductCustomEndpointVerification
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/models"
	return parsed.String(), nil
}

func unavailableCustomEndpointVerification() credentials.VerificationResult {
	return credentials.VerificationResult{
		Status:      credentials.VerificationUnavailable,
		Reason:      credentials.VerificationReasonUnavailable,
		SafeMessage: "provider unavailable",
	}
}
