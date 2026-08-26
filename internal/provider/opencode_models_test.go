package provider

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseOpenCodeModelCatalogCanonicalizesInstalledModels(t *testing.T) {
	models, err := ParseOpenCodeModelCatalog([]byte(strings.Join([]string{
		"zai/glm-5.2",
		"opencode/mimo-v2.5-free",
		"opencode/big-pickle",
		"deepseek/deepseek-chat",
	}, "\n") + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"deepseek/deepseek-chat",
		"opencode/big-pickle",
		"opencode/mimo-v2.5-free",
		"zai/glm-5.2",
	}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
	if selected, ok := SelectOpenCodeNativeModel(models); !ok ||
		selected != "opencode/big-pickle" {
		t.Fatalf("selected = %q, %t", selected, ok)
	}
}

func TestParseOpenCodeModelCatalogFailsClosed(t *testing.T) {
	for _, payload := range [][]byte{
		nil,
		[]byte("opencode/big-pickle\nopencode/big-pickle\n"),
		[]byte("opencode/big-pickle\nnot a model\n"),
		[]byte(" opencode/big-pickle\n"),
		[]byte("opencode/big-pickle\x00\n"),
		[]byte{0xff},
	} {
		if _, err := ParseOpenCodeModelCatalog(payload); !errors.Is(
			err, ErrOpenCodeModelDiscovery,
		) {
			t.Fatalf("payload %q error = %v", payload, err)
		}
	}
	if selected, ok := SelectOpenCodeNativeModel([]string{
		"deepseek/deepseek-chat", "minimax-cn/MiniMax-M3",
	}); ok || selected != "" {
		t.Fatalf("third-party route selected as native: %q, %t", selected, ok)
	}
}
