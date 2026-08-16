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

var ErrInvalidCatalogVerifier = errors.New("invalid Provider catalog verifier")

type credentialVerificationSpec struct {
	URL         string
	Header      string
	HeaderValue string
	ExtraHeader map[string]string
}

var credentialVerificationSpecs = map[string]credentialVerificationSpec{
	"anthropic":       {URL: "https://api.anthropic.com/v1/models", Header: "x-api-key", HeaderValue: "%s", ExtraHeader: map[string]string{"anthropic-version": "2023-06-01"}},
	"google-gemini":   {URL: "https://generativelanguage.googleapis.com/v1beta/models", Header: "x-goog-api-key", HeaderValue: "%s"},
	"deepseek":        {URL: "https://api.deepseek.com/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"kimi":            {URL: "https://api.moonshot.cn/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"minimax":         {URL: "https://api.minimaxi.com/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"xai":             {URL: "https://api.x.ai/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"zhipu":           {URL: "https://open.bigmodel.cn/api/paas/v4/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"alibaba-bailian": {URL: "https://dashscope.aliyuncs.com/compatible-mode/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"tencent-hunyuan": {URL: "https://api.hunyuan.cloud.tencent.com/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"baidu-qianfan":   {URL: "https://qianfan.baidubce.com/v2/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"stepfun":         {URL: "https://api.stepfun.com/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"modelscope":      {URL: "https://api-inference.modelscope.cn/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"openrouter":      {URL: "https://openrouter.ai/api/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"siliconflow":     {URL: "https://api.siliconflow.cn/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"nvidia-nim":      {URL: "https://integrate.api.nvidia.com/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
	"novita":          {URL: "https://api.novita.ai/openai/v1/models", Header: "Authorization", HeaderValue: "Bearer %s"},
}

type CatalogVerifierConfig struct {
	Client           HTTPDoer
	Timeout          time.Duration
	MaxResponseBytes int64
}

type CatalogCredentialVerifier struct {
	client           HTTPDoer
	timeout          time.Duration
	maxResponseBytes int64
}

func SupportsBrokeredCredential(providerID string) bool {
	_, ok := credentialVerificationSpecs[providerID]
	return ok
}

func NewCatalogCredentialVerifier(
	config CatalogVerifierConfig,
) (*CatalogCredentialVerifier, error) {
	if interfaceIsNil(config.Client) || config.Timeout <= 0 ||
		config.MaxResponseBytes < 64 || config.MaxResponseBytes > 1<<20 {
		return nil, ErrInvalidCatalogVerifier
	}
	return &CatalogCredentialVerifier{
		client:           config.Client,
		timeout:          config.Timeout,
		maxResponseBytes: config.MaxResponseBytes,
	}, nil
}

func NewSystemCatalogCredentialVerifier(
	timeout time.Duration,
	maxResponseBytes int64,
) (*CatalogCredentialVerifier, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidCatalogVerifier
	}
	privateTransport := transport.Clone()
	privateTransport.Proxy = nil
	privateTransport.DisableCompression = true
	client := &http.Client{
		Transport: privateTransport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return NewCatalogCredentialVerifier(CatalogVerifierConfig{
		Client: client, Timeout: timeout, MaxResponseBytes: maxResponseBytes,
	})
}

func (verifier *CatalogCredentialVerifier) Verify(
	ctx context.Context,
	providerID string,
	secret []byte,
) (credentials.VerificationResult, error) {
	spec, ok := credentialVerificationSpecs[providerID]
	if verifier == nil || ctx == nil || !ok || len(secret) == 0 || len(secret) > 8192 {
		return credentials.VerificationResult{}, ErrInvalidCatalogVerifier
	}
	requestContext, cancel := context.WithTimeout(ctx, verifier.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, spec.URL, nil)
	if err != nil || !exactVerificationURL(request.URL, spec.URL) {
		return credentials.VerificationResult{}, ErrInvalidCatalogVerifier
	}
	request.Header.Set(spec.Header, formatCredentialHeader(spec.HeaderValue, secret))
	request.Header.Set("Accept", "application/json")
	for name, value := range spec.ExtraHeader {
		request.Header.Set(name, value)
	}
	response, requestErr := verifier.client.Do(request)
	request.Header.Del(spec.Header)
	if requestErr != nil {
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return unavailableVerification(credentials.VerificationReasonTimeout), nil
		}
		return unavailableVerification(credentials.VerificationReasonUnavailable), nil
	}
	if response == nil || response.Body == nil {
		return unavailableVerification(credentials.VerificationReasonUnavailable), nil
	}
	defer response.Body.Close()
	if response.Request != nil && !exactVerificationURL(response.Request.URL, spec.URL) {
		return unavailableVerification(credentials.VerificationReasonUnavailable), nil
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, verifier.maxResponseBytes+1))
	if readErr != nil || int64(len(body)) > verifier.maxResponseBytes {
		return unavailableVerification(credentials.VerificationReasonUnavailable), nil
	}
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		if !json.Valid(body) {
			return unavailableVerification(credentials.VerificationReasonUnavailable), nil
		}
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
		return unavailableVerification(credentials.VerificationReasonUnavailable), nil
	}
}

func formatCredentialHeader(pattern string, secret []byte) string {
	if pattern == "%s" {
		return string(secret)
	}
	return "Bearer " + string(secret)
}

func exactVerificationURL(candidate *url.URL, expected string) bool {
	reference, err := url.Parse(expected)
	return err == nil && candidate != nil &&
		candidate.Scheme == reference.Scheme && candidate.Host == reference.Host &&
		candidate.Path == reference.Path && candidate.RawQuery == reference.RawQuery &&
		candidate.Fragment == ""
}

func unavailableVerification(reason credentials.VerificationReason) credentials.VerificationResult {
	message := "provider unavailable"
	if reason == credentials.VerificationReasonTimeout {
		message = "request timed out"
	}
	return credentials.VerificationResult{
		Status: credentials.VerificationUnavailable, Reason: reason, SafeMessage: message,
	}
}
