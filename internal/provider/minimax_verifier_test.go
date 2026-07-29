package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

type minimaxFixtureDoer struct {
	response *http.Response
	err      error
	request  *http.Request
}

func (doer *minimaxFixtureDoer) Do(
	request *http.Request,
) (*http.Response, error) {
	doer.request = request.Clone(request.Context())
	return doer.response, doer.err
}

func TestMiniMaxVerifierUsesFixedNonGenerativeEndpoint(t *testing.T) {
	doer := &minimaxFixtureDoer{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(
				bytes.NewReader([]byte(`{"models":[]}`)),
			),
			Header: make(http.Header),
		},
	}
	verifier, err := NewMiniMaxCredentialVerifier(MiniMaxVerifierConfig{
		Client:           doer,
		Timeout:          5 * time.Second,
		MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatalf("NewMiniMaxCredentialVerifier() error = %v", err)
	}
	secret := []byte{0x11, 0x22, 0x33, 0x44}
	result, err := verifier.Verify(context.Background(), "minimax", secret)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Status != MiniMaxCredentialValid ||
		result.Reason != MiniMaxCredentialReasonNone {
		t.Fatalf("Verify() = %#v", result)
	}
	if doer.request == nil ||
		doer.request.Method != http.MethodGet ||
		doer.request.URL.Scheme != "https" ||
		doer.request.URL.Host != "api.minimaxi.com" ||
		doer.request.URL.Path != "/v1/models" ||
		doer.request.Header.Get("Authorization") != "Bearer "+string(secret) ||
		doer.request.Body != nil {
		t.Fatalf("request = %#v", doer.request)
	}
}

func TestMiniMaxVerifierRedactsProviderRejection(t *testing.T) {
	doer := &minimaxFixtureDoer{
		response: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body: io.NopCloser(bytes.NewReader([]byte(
				`{"error":"upstream-sensitive-diagnostic"}`,
			))),
			Header: make(http.Header),
		},
	}
	verifier, err := NewMiniMaxCredentialVerifier(MiniMaxVerifierConfig{
		Client:           doer,
		Timeout:          5 * time.Second,
		MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatalf("NewMiniMaxCredentialVerifier() error = %v", err)
	}
	result, err := verifier.Verify(
		context.Background(),
		"minimax",
		[]byte{0x55, 0x66, 0x77, 0x08},
	)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Status != MiniMaxCredentialRejected ||
		result.Reason != MiniMaxCredentialReasonProviderRejected ||
		bytes.Contains([]byte(result.SafeMessage), []byte("sensitive")) {
		t.Fatalf("Verify() = %#v", result)
	}
}

func TestSystemMiniMaxVerifierDisablesProxyAndRedirects(t *testing.T) {
	verifier, err := NewSystemMiniMaxCredentialVerifier(
		5*time.Second,
		4096,
	)
	if err != nil {
		t.Fatalf("NewSystemMiniMaxCredentialVerifier() error = %v", err)
	}
	client, ok := verifier.client.(*http.Client)
	if !ok {
		t.Fatalf("system client = %T", verifier.client)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok ||
		transport.Proxy != nil ||
		!transport.DisableCompression ||
		client.CheckRedirect == nil {
		t.Fatalf("unsafe system client = %#v", client)
	}
	redirect := &http.Request{}
	if redirectErr := client.CheckRedirect(
		redirect,
		[]*http.Request{{}},
	); redirectErr != http.ErrUseLastResponse {
		t.Fatalf("redirect error = %v", redirectErr)
	}
}

func TestMiniMaxVerifierRejectsResponseFromDifferentOrigin(t *testing.T) {
	responseRequest, err := http.NewRequest(
		http.MethodGet,
		"https://example.invalid/v1/models",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	doer := &minimaxFixtureDoer{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(
				bytes.NewReader([]byte(`{"models":[]}`)),
			),
			Header:  make(http.Header),
			Request: responseRequest,
		},
	}
	verifier, err := NewMiniMaxCredentialVerifier(MiniMaxVerifierConfig{
		Client:           doer,
		Timeout:          5 * time.Second,
		MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := verifier.Verify(
		context.Background(),
		"minimax",
		[]byte{0x21, 0x32, 0x43, 0x54},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != MiniMaxCredentialUnavailable ||
		result.Reason != MiniMaxCredentialReasonUnavailable {
		t.Fatalf("Verify() = %#v", result)
	}
}
