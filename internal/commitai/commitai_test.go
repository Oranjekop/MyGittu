package commitai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGenerateUsesStagedPatch(t *testing.T) {
	patch := "diff --git a/file b/file\n+new line"
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNumber := received.Add(1)
		expectedPrompt := "简体中文"
		if requestNumber == 2 {
			expectedPrompt = "Use Conventional Commits"
		}
		var body struct {
			Model    string
			Messages []message
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil || r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" ||
			r.Header.Get("Authorization") != "Bearer test-only" || body.Model != "test-model" ||
			len(body.Messages) != 2 || body.Messages[1].Content != patch ||
			!strings.Contains(body.Messages[0].Content, expectedPrompt) {
			t.Error("unexpected request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"更新文件"}}]}`)
	}))
	defer server.Close()
	config := Config{Provider: "OpenAI-compatible", BaseURL: server.URL + "/v1", Model: "test-model"}
	got, err := Generate(context.Background(), config, "test-only", "zh-CN", patch, "")
	if err != nil || got != "更新文件" {
		t.Fatalf("message=%q error=%v", got, err)
	}
	if _, err := Generate(context.Background(), config, "test-only", "en", patch, "Use Conventional Commits"); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", strings.Repeat("x", MaxPatch+1)} {
		if _, err := Generate(context.Background(), config, "test-only", "en", invalid, ""); err == nil {
			t.Fatal("invalid staged diff accepted")
		}
	}
	if got := received.Load(); got != 2 {
		t.Fatalf("sent %d requests, want 2", got)
	}
}
