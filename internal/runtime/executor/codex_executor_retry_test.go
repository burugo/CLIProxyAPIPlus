package executor

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func TestShouldCompressCodexRequestBody(t *testing.T) {
	t.Run("default upstream", func(t *testing.T) {
		if !shouldCompressCodexRequestBody("") {
			t.Fatal("expected default codex upstream to enable zstd compression")
		}
	})

	t.Run("native codex upstream", func(t *testing.T) {
		if !shouldCompressCodexRequestBody("https://chatgpt.com/backend-api/codex") {
			t.Fatal("expected native codex upstream to enable zstd compression")
		}
	})

	t.Run("gitlab gateway", func(t *testing.T) {
		if shouldCompressCodexRequestBody("https://gitlab.example.com/v1/proxy/openai/v1") {
			t.Fatal("expected gitlab gateway to disable zstd compression")
		}
	})

	t.Run("non codex chatgpt path", func(t *testing.T) {
		if shouldCompressCodexRequestBody("https://chatgpt.com/backend-api/other") {
			t.Fatal("expected non-codex chatgpt path to disable zstd compression")
		}
	})

	t.Run("invalid base url", func(t *testing.T) {
		if shouldCompressCodexRequestBody("://invalid") {
			t.Fatal("expected invalid base url to disable zstd compression")
		}
	})
}

func TestCompressZstdBodyUpdatesGetBodyForRedirectReplays(t *testing.T) {
	var redirectedEncoding string
	var redirectedBody []byte

	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedEncoding = r.Header.Get("Content-Encoding")
		var err error
		redirectedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read redirected body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer final.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	raw := []byte(`{"message":"hello"}`)
	req, err := http.NewRequest(http.MethodPost, redirect.URL, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	compressZstdBody(req)

	resp, err := final.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if redirectedEncoding != "zstd" {
		t.Fatalf("redirected Content-Encoding = %q, want %q", redirectedEncoding, "zstd")
	}

	decoder, err := zstd.NewReader(bytes.NewReader(redirectedBody))
	if err != nil {
		t.Fatalf("zstd.NewReader: %v", err)
	}
	defer decoder.Close()

	decoded, err := io.ReadAll(decoder)
	if err != nil {
		t.Fatalf("decode redirected body: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded redirected body = %q, want %q", decoded, raw)
	}
}

func TestParseCodexRetryAfter(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	t.Run("resets_in_seconds", func(t *testing.T) {
		body := []byte(`{"error":{"type":"usage_limit_reached","resets_in_seconds":123}}`)
		retryAfter := parseCodexRetryAfter(http.StatusTooManyRequests, body, now)
		if retryAfter == nil {
			t.Fatalf("expected retryAfter, got nil")
		}
		if *retryAfter != 123*time.Second {
			t.Fatalf("retryAfter = %v, want %v", *retryAfter, 123*time.Second)
		}
	})

	t.Run("prefers resets_at", func(t *testing.T) {
		resetAt := now.Add(5 * time.Minute).Unix()
		body := []byte(`{"error":{"type":"usage_limit_reached","resets_at":` + itoa(resetAt) + `,"resets_in_seconds":1}}`)
		retryAfter := parseCodexRetryAfter(http.StatusTooManyRequests, body, now)
		if retryAfter == nil {
			t.Fatalf("expected retryAfter, got nil")
		}
		if *retryAfter != 5*time.Minute {
			t.Fatalf("retryAfter = %v, want %v", *retryAfter, 5*time.Minute)
		}
	})

	t.Run("fallback when resets_at is past", func(t *testing.T) {
		resetAt := now.Add(-1 * time.Minute).Unix()
		body := []byte(`{"error":{"type":"usage_limit_reached","resets_at":` + itoa(resetAt) + `,"resets_in_seconds":77}}`)
		retryAfter := parseCodexRetryAfter(http.StatusTooManyRequests, body, now)
		if retryAfter == nil {
			t.Fatalf("expected retryAfter, got nil")
		}
		if *retryAfter != 77*time.Second {
			t.Fatalf("retryAfter = %v, want %v", *retryAfter, 77*time.Second)
		}
	})

	t.Run("non-429 status code", func(t *testing.T) {
		body := []byte(`{"error":{"type":"usage_limit_reached","resets_in_seconds":30}}`)
		if got := parseCodexRetryAfter(http.StatusBadRequest, body, now); got != nil {
			t.Fatalf("expected nil for non-429, got %v", *got)
		}
	})

	t.Run("non usage_limit_reached error type", func(t *testing.T) {
		body := []byte(`{"error":{"type":"server_error","resets_in_seconds":30}}`)
		if got := parseCodexRetryAfter(http.StatusTooManyRequests, body, now); got != nil {
			t.Fatalf("expected nil for non-usage_limit_reached, got %v", *got)
		}
	})
}

func TestNewCodexStatusErrTreatsCapacityAsRetryableRateLimit(t *testing.T) {
	body := []byte(`{"error":{"message":"Selected model is at capacity. Please try a different model."}}`)

	err := newCodexStatusErr(http.StatusBadRequest, body)

	if got := err.StatusCode(); got != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, want %d", got, http.StatusTooManyRequests)
	}
	if err.RetryAfter() != nil {
		t.Fatalf("expected nil explicit retryAfter for capacity fallback, got %v", *err.RetryAfter())
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
