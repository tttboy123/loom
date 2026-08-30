package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

func callProductHarnessControlMCP(
	ctx context.Context,
	url string,
	token string,
	name string,
	arguments json.RawMessage,
	want []byte,
) error {
	body, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"params"`
	}{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}{Name: name, Arguments: arguments},
	})
	if err != nil {
		return harnessadapter.ErrHarnessProtocol
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	responseBody, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK ||
		!bytes.Contains(responseBody, want) {
		return harnessadapter.ErrHarnessProtocol
	}
	return nil
}
