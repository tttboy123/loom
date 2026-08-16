package toolbroker

import (
	"net/http"
	"testing"
	"time"
)

func TestSystemHTTPClientDisablesProxyAndRejectsUnsafeRedirects(t *testing.T) {
	client, err := NewSystemHTTPClient(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || !transport.DisableCompression ||
		transport.DialContext == nil {
		t.Fatalf("transport = %#v", client.Transport)
	}
	unsafe, _ := http.NewRequest(http.MethodGet, "https://127.0.0.1/private", nil)
	if client.CheckRedirect(unsafe, nil) == nil {
		t.Fatal("private redirect was accepted")
	}
	safe, _ := http.NewRequest(http.MethodGet, "https://example.com/docs", nil)
	if err := client.CheckRedirect(safe, nil); err != nil {
		t.Fatalf("public redirect rejected: %v", err)
	}
}
