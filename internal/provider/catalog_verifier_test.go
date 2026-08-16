package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

type catalogVerifierDoer struct {
	request  *http.Request
	response *http.Response
}

func (doer *catalogVerifierDoer) Do(request *http.Request) (*http.Response, error) {
	doer.request = request.Clone(request.Context())
	return doer.response, nil
}

func TestCatalogCredentialVerifierUsesRegisteredEndpointAndHeader(t *testing.T) {
	doer := &catalogVerifierDoer{response: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"data":[]}`)),
		Header:     make(http.Header),
	}}
	verifier, err := NewCatalogCredentialVerifier(CatalogVerifierConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := verifier.Verify(
		context.Background(),
		"deepseek",
		[]byte("secret"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != MiniMaxCredentialValid ||
		doer.request.URL.String() != "https://api.deepseek.com/models" ||
		doer.request.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("result=%#v request=%#v", result, doer.request)
	}
}

func TestCatalogCredentialVerifierRejectsProvidersWithoutFixedEndpoint(t *testing.T) {
	verifier, err := NewCatalogCredentialVerifier(CatalogVerifierConfig{
		Client: &catalogVerifierDoer{}, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"custom-openai", "aws-bedrock", "unknown"} {
		if _, err := verifier.Verify(context.Background(), id, []byte("secret")); err == nil {
			t.Fatalf("Verify(%q) accepted provider without fixed verification endpoint", id)
		}
	}
}
