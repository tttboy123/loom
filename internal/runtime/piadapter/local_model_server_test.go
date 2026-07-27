//go:build unix

package piadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPiLocalModelConfigRequiresExplicitPrivateRoot(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeOf(PiLocalModelServerConfig{}).FieldByName("PrivateRoot")
	if !ok || field.Type.Kind() != reflect.String {
		t.Fatal("PiLocalModelServerConfig must expose an explicit string PrivateRoot")
	}
}

func TestPiLocalModelContextAlignment(t *testing.T) {
	t.Parallel()

	arguments := piLocalModelArguments("/private/model.gguf", "127.0.0.1", 18427)
	values := make(map[string][]string)
	for index := 0; index < len(arguments); index++ {
		if !strings.HasPrefix(arguments[index], "--") {
			continue
		}
		if index+1 < len(arguments) && !strings.HasPrefix(arguments[index+1], "--") {
			values[arguments[index]] = append(values[arguments[index]], arguments[index+1])
			index++
			continue
		}
		values[arguments[index]] = append(values[arguments[index]], "")
	}
	if !reflect.DeepEqual(values["--ctx-size"], []string{"32768"}) ||
		!reflect.DeepEqual(values["--n-predict"], []string{"256"}) {
		t.Fatalf(
			"local model budget = context:%v output:%v, want 32768/256",
			values["--ctx-size"],
			values["--n-predict"],
		)
	}
}

func TestPiLocalModelOwnerCheckRejectsForeignUID(t *testing.T) {
	t.Parallel()

	current := uint32(os.Geteuid())
	foreign := current + 1
	if foreign == current {
		foreign--
	}
	info := piLocalFakeFileInfo{
		system: &syscall.Stat_t{Uid: foreign},
	}
	if piLocalCurrentUserOwns(info) {
		t.Fatal("foreign-owned fixture was accepted")
	}
}

func TestPiLocalModelHealthContextClassification(t *testing.T) {
	t.Run("imminent process exit wins over expired health context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		wait := make(chan error, 1)
		time.AfterFunc(5*time.Millisecond, func() {
			wait <- nil
		})
		if err := classifyPiLocalHealthContext(ctx, wait); !errors.Is(err, ErrPiLocalModelProcess) {
			t.Fatalf("classifyPiLocalHealthContext() error = %v, want process failure", err)
		}
	})

	t.Run("live process preserves health cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		wait := make(chan error, 1)
		if err := classifyPiLocalHealthContext(ctx, wait); !errors.Is(err, ErrPiLocalModelHealth) ||
			!errors.Is(err, context.Canceled) {
			t.Fatalf("classifyPiLocalHealthContext() error = %v, want health cancellation", err)
		}
	})
}

func TestPiLocalModelServerHealthAndCleanup(t *testing.T) {
	for _, variable := range []string{
		"SHOULD_NOT_LEAK",
		"HF_TOKEN",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"ALL_PROXY",
	} {
		t.Setenv(variable, "ambient-secret")
	}
	root := piLocalModelPrivateRoot(t, "local-model")
	executablePath := filepath.Join(root, "llama-server")
	modelPath := filepath.Join(root, "model.gguf")
	port := availablePiLocalModelPort(t)
	script := piLocalModelFixtureScript("ok")
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	model := []byte("bounded-test-model")
	if err := os.WriteFile(modelPath, model, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(model)
	config := PiLocalModelServerConfig{
		PrivateRoot:    root,
		ExecutablePath: executablePath,
		ModelPath:      modelPath,
		Host:           "127.0.0.1",
		Port:           port,
		StartupTimeout: 10 * time.Second,
		CancelGrace:    300 * time.Millisecond,
	}
	server, err := startPiLocalModelServer(
		context.Background(),
		config,
		hex.EncodeToString(digest[:]),
	)
	if err != nil {
		t.Fatalf("startPiLocalModelServer() error = %v", err)
	}
	if want := fmt.Sprintf("http://127.0.0.1:%d/v1", port); server.BaseURL() != want {
		t.Fatalf("BaseURL() = %q, want %q", server.BaseURL(), want)
	}
	if err := server.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := server.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		t.Fatal("local model listener survived Close")
	}
}

func TestPiLocalModelServerFailsClosed(t *testing.T) {
	t.Run("invalid public model digest", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-invalid-digest")
		executablePath := filepath.Join(root, "llama-server")
		modelPath := filepath.Join(root, "model.gguf")
		if err := os.WriteFile(executablePath, []byte(piLocalModelFixtureScript("ok")), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(modelPath, []byte("wrong-model"), 0o600); err != nil {
			t.Fatal(err)
		}
		server, err := StartPiLocalModelServer(context.Background(), PiLocalModelServerConfig{
			PrivateRoot:    root,
			ExecutablePath: executablePath,
			ModelPath:      modelPath,
			Host:           "127.0.0.1",
			Port:           availablePiLocalModelPort(t),
			StartupTimeout: time.Second,
			CancelGrace:    100 * time.Millisecond,
		})
		if !errors.Is(err, ErrInvalidPiLocalModel) || server != nil {
			t.Fatalf("StartPiLocalModelServer() = (%#v, %v), want digest rejection", server, err)
		}
	})

	for _, mode := range []string{"no-health", "bad-status", "bad-body"} {
		t.Run(mode+" cleans process", func(t *testing.T) {
			root := piLocalModelPrivateRoot(t, "local-model-timeout")
			executablePath := filepath.Join(root, "llama-server")
			modelPath := filepath.Join(root, "model.gguf")
			if err := os.WriteFile(executablePath, []byte(piLocalModelFixtureScript(mode)), 0o700); err != nil {
				t.Fatal(err)
			}
			model := []byte("bounded-timeout-model")
			if err := os.WriteFile(modelPath, model, 0o600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(model)
			port := availablePiLocalModelPort(t)
			server, err := startPiLocalModelServer(
				context.Background(),
				PiLocalModelServerConfig{
					PrivateRoot:    root,
					ExecutablePath: executablePath,
					ModelPath:      modelPath,
					Host:           "127.0.0.1",
					Port:           port,
					StartupTimeout: 150 * time.Millisecond,
					CancelGrace:    100 * time.Millisecond,
				},
				hex.EncodeToString(digest[:]),
			)
			if !errors.Is(err, ErrPiLocalModelHealth) || server != nil {
				t.Fatalf("startPiLocalModelServer() = (%#v, %v), want health timeout", server, err)
			}
			connection, dialErr := net.DialTimeout(
				"tcp",
				fmt.Sprintf("127.0.0.1:%d", port),
				100*time.Millisecond,
			)
			if dialErr == nil {
				_ = connection.Close()
				t.Fatal("timed-out local model listener survived")
			}
		})
	}

	t.Run("health then early exit", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-health-exit")
		executablePath := filepath.Join(root, "llama-server")
		modelPath := filepath.Join(root, "model.gguf")
		if err := os.WriteFile(
			executablePath,
			[]byte(piLocalModelFixtureScript("health-then-exit")),
			0o700,
		); err != nil {
			t.Fatal(err)
		}
		model := []byte("bounded-health-exit-model")
		if err := os.WriteFile(modelPath, model, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(model)
		port := availablePiLocalModelPort(t)
		server, err := startPiLocalModelServer(
			context.Background(),
			PiLocalModelServerConfig{
				PrivateRoot:    root,
				ExecutablePath: executablePath,
				ModelPath:      modelPath,
				Host:           "127.0.0.1",
				Port:           port,
				StartupTimeout: 10 * time.Second,
				CancelGrace:    100 * time.Millisecond,
			},
			hex.EncodeToString(digest[:]),
		)
		if !errors.Is(err, ErrPiLocalModelProcess) || server != nil {
			if server != nil {
				_ = server.Close(context.Background())
			}
			t.Fatalf("startPiLocalModelServer() = (%#v, %v), want early-exit rejection", server, err)
		}
	})

	t.Run("occupied port", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-port")
		executablePath := filepath.Join(root, "llama-server")
		modelPath := filepath.Join(root, "model.gguf")
		if err := os.WriteFile(executablePath, []byte(piLocalModelFixtureScript("ok")), 0o700); err != nil {
			t.Fatal(err)
		}
		model := []byte("bounded-port-model")
		if err := os.WriteFile(modelPath, model, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(model)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		port := listener.Addr().(*net.TCPAddr).Port
		server, err := startPiLocalModelServer(
			context.Background(),
			PiLocalModelServerConfig{
				PrivateRoot:    root,
				ExecutablePath: executablePath,
				ModelPath:      modelPath,
				Host:           "127.0.0.1",
				Port:           port,
				StartupTimeout: time.Second,
				CancelGrace:    100 * time.Millisecond,
			},
			hex.EncodeToString(digest[:]),
		)
		if !errors.Is(err, ErrInvalidPiLocalModel) || server != nil {
			t.Fatalf("startPiLocalModelServer() = (%#v, %v), want occupied-port rejection", server, err)
		}
	})

	t.Run("port stolen after first free check", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-port-race")
		executablePath := filepath.Join(root, "llama-server")
		modelPath := filepath.Join(root, "model.gguf")
		if err := os.WriteFile(
			executablePath,
			[]byte(piLocalModelFixtureScript("no-health")),
			0o700,
		); err != nil {
			t.Fatal(err)
		}
		model := []byte("bounded-port-race-model")
		if err := os.WriteFile(modelPath, model, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(model)
		port := availablePiLocalModelPort(t)
		hostPort := fmt.Sprintf("127.0.0.1:%d", port)
		var stolen *http.Server
		server, err := startPiLocalModelServerWithPortGate(
			context.Background(),
			PiLocalModelServerConfig{
				PrivateRoot:    root,
				ExecutablePath: executablePath,
				ModelPath:      modelPath,
				Host:           "127.0.0.1",
				Port:           port,
				StartupTimeout: time.Second,
				CancelGrace:    100 * time.Millisecond,
			},
			hex.EncodeToString(digest[:]),
			func() error {
				listener, listenErr := net.Listen("tcp", hostPort)
				if listenErr != nil {
					return listenErr
				}
				stolen = &http.Server{Handler: http.HandlerFunc(
					func(writer http.ResponseWriter, request *http.Request) {
						if request.URL.Path != "/health" {
							http.NotFound(writer, request)
							return
						}
						writer.Header().Set("Content-Type", "application/json")
						_, _ = writer.Write([]byte(`{"status":"ok"}`))
					},
				)}
				go func() { _ = stolen.Serve(listener) }()
				return nil
			},
		)
		if stolen != nil {
			_ = stolen.Close()
		}
		if !errors.Is(err, ErrInvalidPiLocalModel) || server != nil {
			t.Fatalf("startPiLocalModelServerWithPortGate() = (%#v, %v), want port-race rejection", server, err)
		}
	})

	t.Run("startup cancellation cleans process", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-cancel")
		executablePath := filepath.Join(root, "llama-server")
		modelPath := filepath.Join(root, "model.gguf")
		if err := os.WriteFile(executablePath, []byte(piLocalModelFixtureScript("no-health")), 0o700); err != nil {
			t.Fatal(err)
		}
		model := []byte("bounded-cancel-model")
		if err := os.WriteFile(modelPath, model, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(model)
		port := availablePiLocalModelPort(t)
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)
		server, err := startPiLocalModelServer(
			ctx,
			PiLocalModelServerConfig{
				PrivateRoot:    root,
				ExecutablePath: executablePath,
				ModelPath:      modelPath,
				Host:           "127.0.0.1",
				Port:           port,
				StartupTimeout: time.Second,
				CancelGrace:    100 * time.Millisecond,
			},
			hex.EncodeToString(digest[:]),
		)
		if !errors.Is(err, ErrPiLocalModelHealth) ||
			!errors.Is(err, context.Canceled) ||
			server != nil {
			t.Fatalf("startPiLocalModelServer() = (%#v, %v), want cancellation", server, err)
		}
		connection, dialErr := net.DialTimeout(
			"tcp",
			fmt.Sprintf("127.0.0.1:%d", port),
			100*time.Millisecond,
		)
		if dialErr == nil {
			_ = connection.Close()
			t.Fatal("canceled local model listener survived")
		}
	})

	t.Run("pre-canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		server, err := startPiLocalModelServer(
			ctx,
			PiLocalModelServerConfig{
				PrivateRoot:    "/private/not-opened-root",
				ExecutablePath: "/private/not-opened",
				ModelPath:      "/private/not-opened-model",
				Host:           "127.0.0.1",
				Port:           18427,
				StartupTimeout: time.Second,
				CancelGrace:    100 * time.Millisecond,
			},
			strings.Repeat("a", 64),
		)
		if !errors.Is(err, ErrPiLocalModelProcess) ||
			!errors.Is(err, context.Canceled) ||
			server != nil {
			t.Fatalf("startPiLocalModelServer() = (%#v, %v), want pre-canceled rejection", server, err)
		}
	})
}

func TestPiLocalModelFileBindingsFailClosed(t *testing.T) {
	root := piLocalModelPrivateRoot(t, "local-model-bindings")
	regularPath := filepath.Join(root, "regular")
	if err := os.WriteFile(regularPath, []byte("binding"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("binding"))
	expectedDigest := hex.EncodeToString(digest[:])
	binding, err := bindPiLocalFile(
		regularPath,
		maxPiLocalModelSize,
		false,
		expectedDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regularPath, []byte("binding-drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := revalidatePiLocalBinding(
		binding,
		maxPiLocalModelSize,
		false,
		expectedDigest,
	); !errors.Is(err, ErrPiLocalModelBindingChanged) {
		t.Fatalf("revalidatePiLocalBinding() error = %v, want binding change", err)
	}
	driftDigest := sha256.Sum256([]byte("binding-drift"))
	driftExpected := hex.EncodeToString(driftDigest[:])
	driftBinding, err := bindPiLocalFile(
		regularPath,
		maxPiLocalModelSize,
		false,
		driftExpected,
	)
	if err != nil {
		t.Fatal(err)
	}
	replacementPath := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacementPath, []byte("binding-drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacementPath, regularPath); err != nil {
		t.Fatal(err)
	}
	if err := revalidatePiLocalBinding(
		driftBinding,
		maxPiLocalModelSize,
		false,
		driftExpected,
	); !errors.Is(err, ErrPiLocalModelBindingChanged) {
		t.Fatalf("revalidatePiLocalBinding(replaced path) error = %v, want binding change", err)
	}

	symlinkPath := filepath.Join(root, "symlink")
	if err := os.Symlink(regularPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := bindPiLocalFile(
		symlinkPath,
		maxPiLocalModelSize,
		false,
		"",
	); !errors.Is(err, ErrInvalidPiLocalModel) {
		t.Fatalf("bindPiLocalFile(symlink) error = %v", err)
	}
	if err := os.Chmod(regularPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := bindPiLocalFile(
		regularPath,
		maxPiLocalModelSize,
		false,
		"",
	); !errors.Is(err, ErrInvalidPiLocalModel) {
		t.Fatalf("bindPiLocalFile(mode) error = %v", err)
	}
}

func TestPiLocalModelBindingInspectorIsReadOnlyAndFailClosed(t *testing.T) {
	root := piLocalModelPrivateRoot(t, "local-model-inspector")
	binDirectory := piPrivateDirectoryAt(t, root, "bin")
	modelDirectory := piPrivateDirectoryAt(t, root, "models")
	executablePath := filepath.Join(binDirectory, "llama-server")
	modelPath := filepath.Join(modelDirectory, "model.gguf")
	executable := []byte("#!/bin/sh\nexit 0\n")
	model := []byte("bounded-inspector-model")
	if err := os.WriteFile(executablePath, executable, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modelPath, model, 0o600); err != nil {
		t.Fatal(err)
	}
	expectedModel := sha256.Sum256(model)
	config := PiLocalModelServerConfig{
		PrivateRoot:    root,
		ExecutablePath: executablePath,
		ModelPath:      modelPath,
		Host:           "127.0.0.1",
		Port:           availablePiLocalModelPort(t),
		StartupTimeout: time.Second,
		CancelGrace:    100 * time.Millisecond,
	}
	internal, err := inspectPiLocalModelServerBinding(
		config,
		hex.EncodeToString(expectedModel[:]),
	)
	if err != nil {
		t.Fatalf("inspectPiLocalModelServerBinding() error = %v", err)
	}
	executableDigest := sha256.Sum256(executable)
	if internal.public.ExecutableSHA256 != hex.EncodeToString(executableDigest[:]) ||
		internal.public.ModelSHA256 != hex.EncodeToString(expectedModel[:]) {
		t.Fatalf("binding = %#v", internal.public)
	}
	for _, unexpected := range []string{
		filepath.Join(root, ".llama-home"),
		filepath.Join(root, ".llama-tmp"),
	} {
		if _, err := os.Lstat(unexpected); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read-only inspector created %q", unexpected)
		}
	}

	if err := os.Chmod(binDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := revalidatePiLocalServerBinding(
		internal,
		config,
		hex.EncodeToString(expectedModel[:]),
	); !errors.Is(err, ErrPiLocalModelBindingChanged) {
		t.Fatalf("revalidatePiLocalServerBinding() error = %v, want binding change", err)
	}
}

func TestPiLocalModelBindingInspectorRejectsUnsafePathChainsAndModes(t *testing.T) {
	t.Run("missing explicit root", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-missing-root")
		config, digest := piLocalModelInspectorFixture(t, root)
		config.PrivateRoot = ""
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect without PrivateRoot error = %v", err)
		}
	})

	t.Run("intermediate symlink", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-intermediate-symlink")
		config, digest := piLocalModelInspectorFixture(t, root)
		realDirectory := piPrivateDirectoryAt(t, root, "real-bin")
		linkedDirectory := filepath.Join(root, "linked-bin")
		if err := os.Symlink(realDirectory, linkedDirectory); err != nil {
			t.Fatal(err)
		}
		executablePath := filepath.Join(realDirectory, "llama-server")
		if err := os.Rename(config.ExecutablePath, executablePath); err != nil {
			t.Fatal(err)
		}
		config.ExecutablePath = filepath.Join(linkedDirectory, "llama-server")
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect intermediate symlink error = %v", err)
		}
	})

	t.Run("private root symlink", func(t *testing.T) {
		realRoot := piLocalModelPrivateRoot(t, "local-model-root-target")
		config, digest := piLocalModelInspectorFixture(t, realRoot)
		linkParent := piLocalModelPrivateRoot(t, "local-model-root-link-parent")
		linkedRoot := filepath.Join(linkParent, "linked-root")
		if err := os.Symlink(realRoot, linkedRoot); err != nil {
			t.Fatal(err)
		}
		config.PrivateRoot = linkedRoot
		config.ExecutablePath = filepath.Join(
			linkedRoot,
			"bin",
			filepath.Base(config.ExecutablePath),
		)
		config.ModelPath = filepath.Join(
			linkedRoot,
			"models",
			filepath.Base(config.ModelPath),
		)
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect private-root symlink error = %v", err)
		}
	})

	t.Run("resolved path escape", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-resolved-escape")
		config, digest := piLocalModelInspectorFixture(t, root)
		externalRoot := piLocalModelPrivateRoot(t, "local-model-external")
		externalModel := filepath.Join(externalRoot, "model.gguf")
		model := []byte("bounded-inspector-fixture-model")
		if err := os.WriteFile(externalModel, model, 0o600); err != nil {
			t.Fatal(err)
		}
		linkedDirectory := filepath.Join(root, "external-models")
		if err := os.Symlink(externalRoot, linkedDirectory); err != nil {
			t.Fatal(err)
		}
		config.ModelPath = filepath.Join(linkedDirectory, "model.gguf")
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect resolved escape error = %v", err)
		}
	})

	t.Run("lexical path escape", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-lexical-escape")
		config, digest := piLocalModelInspectorFixture(t, root)
		externalRoot := piLocalModelPrivateRoot(t, "local-model-lexical-external")
		externalModel := filepath.Join(externalRoot, "model.gguf")
		model := []byte("bounded-inspector-fixture-model")
		if err := os.WriteFile(externalModel, model, 0o600); err != nil {
			t.Fatal(err)
		}
		config.ModelPath = externalModel
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect lexical escape error = %v", err)
		}
	})

	t.Run("writable pre-root ancestor", func(t *testing.T) {
		userHome, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		publicParent, err := os.MkdirTemp(userHome, ".loom-public-parent-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(publicParent) })
		if err := os.Chmod(publicParent, 0o777); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(publicParent, "private")
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
		config, digest := piLocalModelInspectorFixture(t, root)
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect writable ancestor error = %v", err)
		}
	})

	t.Run("descendant directory mode", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-directory-mode")
		config, digest := piLocalModelInspectorFixture(t, root)
		if err := os.Chmod(filepath.Dir(config.ExecutablePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect descendant mode error = %v", err)
		}
	})

	t.Run("executable exact mode", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-executable-mode")
		config, digest := piLocalModelInspectorFixture(t, root)
		if err := os.Chmod(config.ExecutablePath, 0o500); err != nil {
			t.Fatal(err)
		}
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect executable mode error = %v", err)
		}
	})

	t.Run("model exact mode", func(t *testing.T) {
		root := piLocalModelPrivateRoot(t, "local-model-model-mode")
		config, digest := piLocalModelInspectorFixture(t, root)
		if err := os.Chmod(config.ModelPath, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := inspectPiLocalModelServerBinding(config, digest); !errors.Is(err, ErrInvalidPiLocalModel) {
			t.Fatalf("inspect model mode error = %v", err)
		}
	})
}

func piLocalModelInspectorFixture(
	t testing.TB,
	root string,
) (PiLocalModelServerConfig, string) {
	t.Helper()
	binDirectory := piPrivateDirectoryAt(t, root, "bin")
	modelDirectory := piPrivateDirectoryAt(t, root, "models")
	executablePath := filepath.Join(binDirectory, "llama-server")
	modelPath := filepath.Join(modelDirectory, "model.gguf")
	if err := os.WriteFile(executablePath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	model := []byte("bounded-inspector-fixture-model")
	if err := os.WriteFile(modelPath, model, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(model)
	return PiLocalModelServerConfig{
		PrivateRoot:    root,
		ExecutablePath: executablePath,
		ModelPath:      modelPath,
		Host:           "127.0.0.1",
		Port:           availablePiLocalModelPort(t),
		StartupTimeout: time.Second,
		CancelGrace:    100 * time.Millisecond,
	}, hex.EncodeToString(digest[:])
}

func piLocalModelPrivateRoot(t testing.TB, name string) string {
	t.Helper()
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(userHome, ".loom-test-private")
	if err := os.Mkdir(parent, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(parent, name+"-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(root)
	})
	return root
}

type piLocalFakeFileInfo struct {
	system any
}

func (piLocalFakeFileInfo) Name() string       { return "foreign" }
func (piLocalFakeFileInfo) Size() int64        { return 1 }
func (piLocalFakeFileInfo) Mode() os.FileMode  { return 0o600 }
func (piLocalFakeFileInfo) ModTime() time.Time { return time.Time{} }
func (piLocalFakeFileInfo) IsDir() bool        { return false }
func (info piLocalFakeFileInfo) Sys() any      { return info.system }

func availablePiLocalModelPort(t testing.TB) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func piLocalModelFixtureScript(mode string) string {
	header := `#!/bin/sh
[ -z "${SHOULD_NOT_LEAK+x}" ] || exit 41
[ -z "${HF_TOKEN+x}" ] || exit 42
[ -z "${HTTP_PROXY+x}" ] || exit 63
[ -z "${HTTPS_PROXY+x}" ] || exit 64
[ -z "${ALL_PROXY+x}" ] || exit 65
[ "$NO_PROXY" = "127.0.0.1" ] || exit 66
[ "$no_proxy" = "127.0.0.1" ] || exit 67
[ "$#" -eq 28 ] || exit 43
[ "$1" = "--model" ] || exit 44
model="$2"
[ -f "$model" ] || exit 45
shift 2
[ "$1" = "--alias" ] && [ "$2" = "qwen2.5-coder-1.5b-instruct-q4-k-m" ] || exit 46
shift 2
[ "$1" = "--host" ] && [ "$2" = "127.0.0.1" ] || exit 47
shift 2
[ "$1" = "--port" ] || exit 48
port="$2"
shift 2
[ "$1" = "--ctx-size" ] && [ "$2" = "32768" ] || exit 49
shift 2
[ "$1" = "--parallel" ] && [ "$2" = "1" ] || exit 50
shift 2
[ "$1" = "--threads" ] && [ "$2" = "4" ] || exit 51
shift 2
[ "$1" = "--n-predict" ] && [ "$2" = "256" ] || exit 52
shift 2
[ "$1" = "--gpu-layers" ] && [ "$2" = "all" ] || exit 53
shift 2
[ "$1" = "--offline" ] || exit 54
shift
[ "$1" = "--no-webui" ] || exit 55
shift
[ "$1" = "--no-agent" ] || exit 56
shift
[ "$1" = "--no-ui-mcp-proxy" ] || exit 57
shift
[ "$1" = "--no-context-shift" ] || exit 58
shift
[ "$1" = "--no-cache-prompt" ] || exit 59
shift
[ "$1" = "--sse-ping-interval" ] && [ "$2" = "-1" ] || exit 60
shift 2
[ "$1" = "--timeout" ] && [ "$2" = "30" ] || exit 61
shift 2
[ "$#" -eq 0 ] || exit 62
`
	if mode == "no-health" {
		return header + "exec /bin/sleep 30\n"
	}
	status := 200
	body := `{"status":"ok"}`
	serverLoop := "serve_forever()"
	switch mode {
	case "ok":
	case "health-then-exit":
		serverLoop = "handle_request()"
	case "bad-status":
		status = 503
	case "bad-body":
		body = `{"status":"warming"}`
	default:
		panic("unknown local model fixture mode")
	}
	return header + fmt.Sprintf(`exec /usr/bin/python3 -c '
import http.server, json, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/health":
            self.send_response(404); self.end_headers(); return
        body = %q.encode("utf-8")
        self.send_response(%d)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, fmt, *args):
        pass
http.server.HTTPServer(("127.0.0.1", int(sys.argv[1])), H).%s
' "$port"
`, body, status, serverLoop)
}
