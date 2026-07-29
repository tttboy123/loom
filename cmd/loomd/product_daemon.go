package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"

	_ "modernc.org/sqlite"
)

const localProductBuildID = "loom-phase2a-w1"

type productDaemonFailure struct {
	code string
	err  error
}

func (failure *productDaemonFailure) Error() string {
	return "product daemon failed: " + failure.code
}

func (failure *productDaemonFailure) Unwrap() error {
	return failure.err
}

func (failure *productDaemonFailure) DaemonFailureCode() string {
	return failure.code
}

func classifyProductDaemonFailure(code string, err error) error {
	if err == nil {
		err = errors.New("product daemon lifecycle failure")
	}
	return &productDaemonFailure{code: code, err: err}
}

type productDaemonRunner struct {
	observer daemonRunner
	server   *localipc.Server
	database *sql.DB

	mu      sync.Mutex
	running bool
	closed  bool
}

func newProductDaemonRunner(
	observer daemonRunner,
	statePath,
	socketPath string,
) (_ *productDaemonRunner, resultErr error) {
	if observer == nil {
		return nil, errors.New("invalid product daemon")
	}
	database, err := openProductReadDatabase(statePath)
	if err != nil {
		_ = observer.Close()
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = database.Close()
			_ = observer.Close()
		}
	}()
	readModel := projection.New(database)
	service, err := api.NewLocalProductReadService(api.LocalProductReadConfig{
		Journal:    journal.NewStore(database),
		Projection: readModel,
		Now: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, err
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      localProductBuildID,
		Handler: localipc.HandlerFunc(
			localProductHandler(service),
		),
	})
	if err != nil {
		return nil, err
	}
	return &productDaemonRunner{
		observer: observer,
		server:   server,
		database: database,
	}, nil
}

func (runner *productDaemonRunner) Run(
	ctx context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	if runner == nil || ctx == nil {
		return app.LocalRuntimeObservationDaemonResult{},
			errors.New("invalid product daemon")
	}
	runner.mu.Lock()
	if runner.running || runner.closed {
		runner.mu.Unlock()
		return app.LocalRuntimeObservationDaemonResult{},
			errors.New("product daemon unavailable")
	}
	runner.running = true
	runner.mu.Unlock()
	defer func() {
		runner.mu.Lock()
		runner.running = false
		runner.mu.Unlock()
	}()

	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- runner.server.Serve(runContext)
	}()
	select {
	case <-runner.server.Ready():
	case serverErr := <-serverDone:
		return app.LocalRuntimeObservationDaemonResult{},
			classifyProductDaemonFailure("local_ipc", serverErr)
	case <-ctx.Done():
		closeErr := runner.server.Close()
		serverErr := <-serverDone
		if closeErr != nil || serverErr != nil {
			return app.LocalRuntimeObservationDaemonResult{},
				classifyProductDaemonFailure(
					"shutdown",
					errors.Join(serverErr, closeErr),
				)
		}
		return app.LocalRuntimeObservationDaemonResult{}, ctx.Err()
	}

	type observerOutcome struct {
		result app.LocalRuntimeObservationDaemonResult
		err    error
	}
	observerDone := make(chan observerOutcome, 1)
	go func() {
		result, err := runner.observer.Run(runContext)
		observerDone <- observerOutcome{result: result, err: err}
	}()

	select {
	case outcome := <-observerDone:
		cancel()
		closeErr := runner.server.Close()
		serverErr := <-serverDone
		if outcome.err != nil &&
			!errors.Is(outcome.err, context.Canceled) &&
			!errors.Is(outcome.err, context.DeadlineExceeded) {
			return outcome.result, classifyProductDaemonFailure(
				"observer",
				errors.Join(outcome.err, serverErr, closeErr),
			)
		}
		if serverErr != nil || closeErr != nil {
			return outcome.result, classifyProductDaemonFailure(
				"shutdown",
				errors.Join(serverErr, closeErr),
			)
		}
		return outcome.result, outcome.err
	case serverErr := <-serverDone:
		cancel()
		outcome := <-observerDone
		return outcome.result, classifyProductDaemonFailure(
			"local_ipc",
			errors.Join(serverErr, outcome.err),
		)
	case <-ctx.Done():
		cancel()
		closeErr := runner.server.Close()
		outcome := <-observerDone
		serverErr := <-serverDone
		if outcome.err != nil &&
			!errors.Is(outcome.err, context.Canceled) &&
			!errors.Is(outcome.err, context.DeadlineExceeded) {
			return outcome.result, classifyProductDaemonFailure(
				"observer",
				errors.Join(outcome.err, serverErr, closeErr),
			)
		}
		if serverErr != nil || closeErr != nil {
			return outcome.result, classifyProductDaemonFailure(
				"shutdown",
				errors.Join(serverErr, closeErr),
			)
		}
		return outcome.result, ctx.Err()
	}
}

func (runner *productDaemonRunner) Close() error {
	if runner == nil {
		return errors.New("invalid product daemon")
	}
	runner.mu.Lock()
	if runner.running {
		runner.mu.Unlock()
		return errors.New("product daemon running")
	}
	if runner.closed {
		runner.mu.Unlock()
		return nil
	}
	runner.closed = true
	runner.mu.Unlock()
	return errors.Join(
		runner.server.Close(),
		runner.database.Close(),
		runner.observer.Close(),
	)
}

func localProductHandler(
	service *api.LocalProductReadService,
) func(context.Context, localipc.Request) localipc.Response {
	return func(
		ctx context.Context,
		request localipc.Request,
	) localipc.Response {
		switch request.Method {
		case "snapshot":
			var input api.LocalProductSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductRequest,
				)
			}
			result, err := service.ReadLocalProductSnapshot(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "timeline_page":
			var input api.LocalProductTimelineRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductRequest,
				)
			}
			result, err := service.ReadLocalProductTimeline(ctx, input)
			if err != nil {
				var gapErr *api.TimelineGapError
				if !errors.As(err, &gapErr) {
					return productServiceError(err)
				}
			}
			return productResultResponse(result)
		default:
			return productErrorResponse(
				"unknown_method",
				errors.New("unknown method"),
			)
		}
	}
}

func decodeExactProductParams(data []byte, output any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing product params")
	}
	return nil
}

func productResultResponse(value any) localipc.Response {
	result, err := json.Marshal(value)
	if err != nil {
		return productErrorResponse("internal", err)
	}
	return localipc.Response{OK: true, Result: result}
}

func productServiceError(err error) localipc.Response {
	switch {
	case errors.Is(err, api.ErrInvalidLocalProductRequest),
		errors.Is(err, api.ErrInvalidTimelineRequest):
		return productErrorResponse("invalid_request", err)
	case errors.Is(err, api.ErrTeamTimelineNotFound):
		return productErrorResponse("not_found", err)
	case errors.Is(err, api.ErrTimelineCursorConflict):
		return productErrorResponse("cursor_conflict", err)
	case errors.Is(err, api.ErrStreamGap):
		return productErrorResponse("stream_gap", err)
	case errors.Is(err, context.DeadlineExceeded):
		return productErrorResponse("timeout", err)
	default:
		return productErrorResponse("state_unavailable", err)
	}
}

func productErrorResponse(code string, cause error) localipc.Response {
	return localipc.Response{
		OK:    false,
		Error: localipcSafeError(code, cause),
	}
}

func localipcSafeError(code string, _ error) *localipc.ProtocolError {
	messages := map[string]struct {
		message     string
		recoverable bool
	}{
		"invalid_request":   {"invalid request", false},
		"unknown_method":    {"unknown method", false},
		"not_found":         {"not found", false},
		"cursor_conflict":   {"cursor conflict", true},
		"stream_gap":        {"stream gap", true},
		"state_unavailable": {"state unavailable", true},
		"timeout":           {"request timed out", true},
		"internal":          {"internal error", true},
	}
	definition, ok := messages[code]
	if !ok {
		code = "internal"
		definition = messages[code]
	}
	return &localipc.ProtocolError{
		Code:        code,
		Message:     definition.message,
		Recoverable: definition.recoverable,
	}
}

func openProductReadDatabase(statePath string) (*sql.DB, error) {
	absolute, err := filepath.Abs(statePath)
	if err != nil || !filepath.IsAbs(absolute) {
		return nil, errors.New("state unavailable")
	}
	info, err := os.Stat(absolute)
	if err != nil || info.IsDir() {
		return nil, errors.New("state unavailable")
	}
	values := url.Values{}
	values.Add("mode", "ro")
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "query_only(1)")
	uri := url.URL{Scheme: "file", Path: absolute}
	uri.RawQuery = values.Encode()
	database, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, errors.New("state unavailable")
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, errors.New("state unavailable")
	}
	return database, nil
}
