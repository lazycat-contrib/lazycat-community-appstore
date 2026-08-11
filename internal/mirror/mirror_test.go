package mirror

import "testing"

func TestOrderedForURLPinsPreferredAndPreservesConfiguredOrder(t *testing.T) {
	entries := []Entry{
		{ID: "download-a", Kind: KindDownload, Name: "A", URL: "https://a.example/https://github.com"},
		{ID: "raw-a", Kind: KindRaw, Name: "Raw", URL: "https://raw.example/https://raw.githubusercontent.com"},
		{ID: "download-b", Kind: KindDownload, Name: "B", URL: "https://b.example/https://github.com"},
		{ID: "download-c", Kind: KindDownload, Name: "C", URL: "https://c.example/https://github.com"},
	}

	ordered := OrderedForURL(entries, "https://github.com/acme/app/releases/download/v1/app.lpk", "download-b")
	if got, want := mirrorIDs(ordered), "download-b,download-a,download-c"; got != want {
		t.Fatalf("ordered IDs = %q, want %q", got, want)
	}
	if got, want := mirrorIDs(entries), "download-a,raw-a,download-b,download-c"; got != want {
		t.Fatalf("input order mutated: %q, want %q", got, want)
	}
}

func TestResolveSelectionUsesPreferredThenStableFallback(t *testing.T) {
	entries := []Entry{
		{ID: "download-a", Kind: KindDownload, Name: "A", URL: "https://a.example/https://github.com"},
		{ID: "download-b", Kind: KindDownload, Name: "B", URL: "https://b.example/https://github.com"},
	}
	upstream := "https://github.com/acme/app/releases/download/v1/app.lpk"

	resolved, ok := ResolveSelection(entries, upstream, PreferredSelection, "download-b")
	if !ok || resolved != "https://b.example/https://github.com/acme/app/releases/download/v1/app.lpk" {
		t.Fatalf("preferred resolution = %q, %t", resolved, ok)
	}
	resolved, ok = ResolveSelection(entries, upstream, PreferredSelection, "missing")
	if !ok || resolved != "https://a.example/https://github.com/acme/app/releases/download/v1/app.lpk" {
		t.Fatalf("stable fallback resolution = %q, %t", resolved, ok)
	}
	resolved, ok = ResolveSelection(nil, upstream, PreferredSelection, "")
	if !ok || resolved != upstream {
		t.Fatalf("direct fallback resolution = %q, %t", resolved, ok)
	}
}

func mirrorIDs(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	result := entries[0].ID
	for _, entry := range entries[1:] {
		result += "," + entry.ID
	}
	return result
}
