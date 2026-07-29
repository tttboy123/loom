package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"loom-pi-rebuild/internal/credentials"
)

var ErrInvalidMiniMaxVerifier = errors.New("invalid MiniMax verifier")

type MiniMaxCredentialStatus = credentials.VerificationStatus
type MiniMaxCredentialReason = credentials.VerificationReason
type MiniMaxCredentialResult = credentials.VerificationResult

const (
	MiniMaxCredentialValid       = credentials.VerificationValid
	MiniMaxCredentialRejected    = credentials.VerificationRejected
	MiniMaxCredentialUnavailable = credentials.VerificationUnavailable

	MiniMaxCredentialReasonNone             = credentials.VerificationReasonNone
	MiniMaxCredentialReasonProviderRejected = credentials.VerificationReasonProviderRejected
	MiniMaxCredentialReasonUnavailable      = credentials.VerificationReasonUnavailable
	MiniMaxCredentialReasonTimeout          = credentials.VerificationReasonTimeout
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type MiniMaxVerifierConfig struct {
	Client           HTTPDoer
	Timeout          time.Duration
	MaxResponseBytes int64
}

type MiniMaxCredentialVerifier struct {
	client           HTTPDoer
	timeout          time.Duration
	maxResponseBytes int64
}

func NewMiniMaxCredentialVerifier(
	config MiniMaxVerifierConfig,
) (*MiniMaxCredentialVerifier, error) {
	if interfaceIsNil(config.Client) ||
		config.Timeout <= 0 ||
		config.MaxResponseBytes < 64 ||
		config.MaxResponseBytes > 1<<20 {
		return nil, ErrInvalidMiniMaxVerifier
	}
	return &MiniMaxCredentialVerifier{
		client:           config.Client,
		timeout:          config.Timeout,
		maxResponseBytes: config.MaxResponseBytes,
	}, nil
}

func NewSystemMiniMaxCredentialVerifier(
	timeout time.Duration,
	maxResponseBytes int64,
) (*MiniMaxCredentialVerifier, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidMiniMaxVerifier
	}
	privateTransport := transport.Clone()
	privateTransport.Proxy = nil
	privateTransport.DisableCompression = true
	client := &http.Client{
		Transport: privateTransport,
		Timeout:   timeout,
		CheckRedirect: func(
			_ *http.Request,
			_ []*http.Request,
		) error {
			return http.ErrUseLastResponse
		},
	}
	return NewMiniMaxCredentialVerifier(MiniMaxVerifierConfig{
		Client:           client,
		Timeout:          timeout,
		MaxResponseBytes: maxResponseBytes,
	})
}

func (verifier *MiniMaxCredentialVerifier) Verify(
	ctx context.Context,
	providerID string,
	secret []byte,
) (credentials.VerificationResult, error) {
	if verifier == nil ||
		ctx == nil ||
		providerID != "minimax" ||
		len(secret) == 0 ||
		len(secret) > 8192 {
		return credentials.VerificationResult{}, ErrInvalidMiniMaxVerifier
	}
	requestContext, cancel := context.WithTimeout(ctx, verifier.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext,
		http.MethodGet,
		"https://api.minimaxi.com/v1/models",
		nil,
	)
	if err != nil {
		return credentials.VerificationResult{}, ErrInvalidMiniMaxVerifier
	}
	if !isExactMiniMaxOrigin(request.URL) {
		return credentials.VerificationResult{}, ErrInvalidMiniMaxVerifier
	}
	request.Header.Set("Authorization", "Bearer "+string(secret))
	request.Header.Set("Accept", "application/json")
	response, err := verifier.client.Do(request)
	request.Header.Del("Authorization")
	if err != nil {
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return credentials.VerificationResult{
				Status:      MiniMaxCredentialUnavailable,
				Reason:      MiniMaxCredentialReasonTimeout,
				SafeMessage: "request timed out",
			}, nil
		}
		return credentials.VerificationResult{
			Status:      MiniMaxCredentialUnavailable,
			Reason:      MiniMaxCredentialReasonUnavailable,
			SafeMessage: "provider unavailable",
		}, nil
	}
	if response == nil || response.Body == nil {
		return credentials.VerificationResult{
			Status:      MiniMaxCredentialUnavailable,
			Reason:      MiniMaxCredentialReasonUnavailable,
			SafeMessage: "provider unavailable",
		}, nil
	}
	defer response.Body.Close()
	if response.Request != nil &&
		!isExactMiniMaxOrigin(response.Request.URL) {
		return credentials.VerificationResult{
			Status:      MiniMaxCredentialUnavailable,
			Reason:      MiniMaxCredentialReasonUnavailable,
			SafeMessage: "provider unavailable",
		}, nil
	}
	body, readErr := io.ReadAll(io.LimitReader(
		response.Body,
		verifier.maxResponseBytes+1,
	))
	if readErr != nil || int64(len(body)) > verifier.maxResponseBytes {
		return credentials.VerificationResult{
			Status:      MiniMaxCredentialUnavailable,
			Reason:      MiniMaxCredentialReasonUnavailable,
			SafeMessage: "provider unavailable",
		}, nil
	}
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		if !json.Valid(body) {
			return credentials.VerificationResult{
				Status:      MiniMaxCredentialUnavailable,
				Reason:      MiniMaxCredentialReasonUnavailable,
				SafeMessage: "provider unavailable",
			}, nil
		}
		return credentials.VerificationResult{
			Status:      MiniMaxCredentialValid,
			Reason:      MiniMaxCredentialReasonNone,
			SafeMessage: "credential verified",
		}, nil
	case response.StatusCode == http.StatusUnauthorized ||
		response.StatusCode == http.StatusForbidden:
		return credentials.VerificationResult{
			Status:      MiniMaxCredentialRejected,
			Reason:      MiniMaxCredentialReasonProviderRejected,
			SafeMessage: "credential rejected",
		}, nil
	default:
		return credentials.VerificationResult{
			Status:      MiniMaxCredentialUnavailable,
			Reason:      MiniMaxCredentialReasonUnavailable,
			SafeMessage: "provider unavailable",
		}, nil
	}
}

func isExactMiniMaxOrigin(target *url.URL) bool {
	return target != nil &&
		target.Scheme == "https" &&
		target.Host == "api.minimaxi.com" &&
		target.Path == "/v1/models" &&
		target.RawQuery == "" &&
		target.Fragment == ""
}
