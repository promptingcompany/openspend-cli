package cmd

import (
	"testing"

	"github.com/promptingcompany/openspend-cli/internal/api"
)

func TestInvocationURLForItem_PrefersInvokeURL(t *testing.T) {
	item := api.SearchResultItem{
		ID:        "res-123",
		InvokeURL: "https://openspend.ai/api/x402/p/res-123",
	}

	got := invocationURLForItem("https://openspend.ai", item)
	if got != item.InvokeURL {
		t.Fatalf("expected invoke URL %q, got %q", item.InvokeURL, got)
	}
}

func TestInvocationURLForItem_FallbackFromBaseURL(t *testing.T) {
	item := api.SearchResultItem{ID: "res-123"}

	got := invocationURLForItem("https://openspend.ai", item)
	want := "https://openspend.ai/api/x402/p/res-123"
	if got != want {
		t.Fatalf("expected fallback invoke URL %q, got %q", want, got)
	}
}

func TestInvocationURLForItem_FallbackFromBaseURLWithPath(t *testing.T) {
	item := api.SearchResultItem{ID: "res-123"}

	got := invocationURLForItem("https://example.com/platform/", item)
	want := "https://example.com/platform/api/x402/p/res-123"
	if got != want {
		t.Fatalf("expected fallback invoke URL %q, got %q", want, got)
	}
}

func TestInvocationURLForItem_InvalidInput(t *testing.T) {
	if got := invocationURLForItem("", api.SearchResultItem{ID: "res-123"}); got != "" {
		t.Fatalf("expected empty invoke URL for empty base URL, got %q", got)
	}
	if got := invocationURLForItem("not-a-url", api.SearchResultItem{ID: "res-123"}); got != "" {
		t.Fatalf("expected empty invoke URL for invalid base URL, got %q", got)
	}
	if got := invocationURLForItem("https://openspend.ai", api.SearchResultItem{}); got != "" {
		t.Fatalf("expected empty invoke URL for empty item ID, got %q", got)
	}
}
