package localipc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync/atomic"
	"time"
)

type ClientConfig struct {
	SocketPath      string
	Timeout         time.Duration
	ExtendedTimeout time.Duration
}

type Client struct {
	socketPath      string
	timeout         time.Duration
	extendedTimeout time.Duration
	requestIDPrefix string
	nextID          atomic.Uint64
}

func NewClient(config ClientConfig) (*Client, error) {
	explicitExtendedTimeout := config.ExtendedTimeout != 0
	if config.ExtendedTimeout == 0 {
		config.ExtendedTimeout = config.Timeout
	}
	if config.Timeout <= 0 ||
		config.ExtendedTimeout < config.Timeout ||
		explicitExtendedTimeout &&
			config.ExtendedTimeout <= extendedResponseDeadline ||
		validateSocketPath(config.SocketPath, os.Geteuid()) != nil {
		return nil, ErrInvalidSocketPath
	}
	requestIDPrefix, err := newClientRequestIDPrefix()
	if err != nil {
		return nil, err
	}
	return &Client{
		socketPath:      config.SocketPath,
		timeout:         config.Timeout,
		extendedTimeout: config.ExtendedTimeout,
		requestIDPrefix: requestIDPrefix,
	}, nil
}

func newClientRequestIDPrefix() (string, error) {
	var nonce [8]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return "", ErrInvalidProtocol
	}
	return "loom-client-" + hex.EncodeToString(nonce[:]), nil
}

func (client *Client) Ping(ctx context.Context) (PingResult, error) {
	var result PingResult
	if err := client.Call(ctx, "ping", struct{}{}, &result); err != nil {
		return PingResult{}, err
	}
	if result.ProtocolVersion != protocolVersion ||
		result.BuildID == "" {
		return PingResult{}, ErrInvalidProtocol
	}
	return result, nil
}

func (client *Client) Call(
	ctx context.Context,
	method string,
	params any,
	result any,
) error {
	return client.call(ctx, "", method, params, result)
}

func (client *Client) call(
	ctx context.Context,
	journeyID string,
	method string,
	params any,
	result any,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validMethod(method) || params == nil || result == nil {
		return ErrInvalidProtocol
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil || len(paramsBytes) == 0 || paramsBytes[0] != '{' {
		return ErrInvalidProtocol
	}
	request := Request{
		Version: protocolVersion,
		RequestID: client.requestIDPrefix + "-" +
			decimalRequestID(client.nextID.Add(1)),
		Method:    method,
		Params:    paramsBytes,
		JourneyID: journeyID,
	}
	body, err := json.Marshal(request)
	if err != nil || hasDuplicateJSONKeys(body) {
		return ErrInvalidProtocol
	}
	deadline := time.Now().Add(client.timeoutForMethod(method))
	if contextDeadline, ok := ctx.Deadline(); ok &&
		contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	dialer := net.Dialer{Timeout: time.Until(deadline)}
	connection, err := dialer.DialContext(ctx, "unix", client.socketPath)
	if err != nil {
		return ErrLocalProductUnavailable
	}
	unixConnection, ok := connection.(*net.UnixConn)
	if !ok {
		_ = connection.Close()
		return ErrInvalidProtocol
	}
	defer unixConnection.Close()
	if err := unixConnection.SetDeadline(deadline); err != nil {
		return ErrLocalProductUnavailable
	}
	stopCancellation := context.AfterFunc(ctx, func() {
		_ = unixConnection.SetDeadline(time.Now())
		_ = unixConnection.Close()
	})
	defer stopCancellation()
	if err := writeFrame(
		unixConnection,
		body,
		maxRequestBodyBytes,
	); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrLocalProductUnavailable
	}
	if err := unixConnection.CloseWrite(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrLocalProductUnavailable
	}
	responseBytes, err := readSingleFrame(
		unixConnection,
		maxResponseBodyBytes,
	)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var networkErr net.Error
		if errors.Is(err, os.ErrDeadlineExceeded) ||
			errors.As(err, &networkErr) && networkErr.Timeout() {
			return ErrProtocolTimeout
		}
		return ErrLocalProductUnavailable
	}
	if hasDuplicateJSONKeys(responseBytes) {
		return ErrInvalidProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(responseBytes))
	decoder.DisallowUnknownFields()
	var response Response
	if err := decoder.Decode(&response); err != nil ||
		decoder.Decode(&struct{}{}) != io.EOF ||
		response.Version != protocolVersion ||
		!validRequestID(response.RequestID) ||
		response.RequestID != request.RequestID ||
		response.JourneyID != journeyID ||
		response.OK == (response.Error != nil) {
		return ErrInvalidProtocol
	}
	if !response.OK {
		return &RemoteError{
			Code:        response.Error.Code,
			Recoverable: response.Error.Recoverable,
			Stage:       response.Error.Stage,
			IncidentID:  response.RequestID,
		}
	}
	if len(response.Result) == 0 ||
		json.Unmarshal(response.Result, result) != nil {
		return ErrInvalidProtocol
	}
	return nil
}

func (client *Client) timeoutForMethod(method string) time.Duration {
	if method == "mission_execution" {
		return missionResponseDeadline + 3*time.Second
	}
	if method == "chat_message" {
		return chatResponseDeadline + 3*time.Second
	}
	if method == "agent_attempt_recovery" {
		return agentRecoveryResponse + 3*time.Second
	}
	if method == "roundtable_steer_seat" {
		return roundtableSteerResponse + 3*time.Second
	}
	if usesExtendedRequestDeadline(method) {
		return client.extendedTimeout
	}
	return client.timeout
}

func (client *Client) CallJourney(
	ctx context.Context,
	journeyID string,
	method string,
	params any,
	result any,
) error {
	if !requiresJourney(method) || !validJourneyID(journeyID) {
		return ErrInvalidProtocol
	}
	return client.call(ctx, journeyID, method, params, result)
}

var ErrLocalProductUnavailable = errors.New("local product unavailable")

type RemoteError struct {
	Code        string
	Recoverable bool
	Stage       string
	IncidentID  string
}

func (err *RemoteError) Error() string {
	return "local product request failed: " + err.Code
}

func decimalRequestID(value uint64) string {
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}
