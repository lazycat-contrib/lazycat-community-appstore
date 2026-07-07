package pagination

import (
	"net/http/httptest"
	"testing"
)

func TestFromRequestUsesConfiguredDefaultPageSize(t *testing.T) {
	req := httptest.NewRequest("GET", "/items", nil)
	params := FromRequest(req, 48, 100)

	if params.Page != 1 {
		t.Fatalf("page = %d, want 1", params.Page)
	}
	if params.PageSize != 48 {
		t.Fatalf("page size = %d, want 48", params.PageSize)
	}
}

func TestFromRequestClampsPageSize(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?page=2&pageSize=500", nil)
	params := FromRequest(req, 24, 100)

	if params.Page != 2 {
		t.Fatalf("page = %d, want 2", params.Page)
	}
	if params.PageSize != 100 {
		t.Fatalf("page size = %d, want 100", params.PageSize)
	}
}

func TestFromRequestFallsBackWhenConfiguredDefaultInvalid(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?pageSize=0", nil)
	params := FromRequest(req, 0, 100)

	if params.PageSize != DefaultPageSize {
		t.Fatalf("page size = %d, want %d", params.PageSize, DefaultPageSize)
	}
}
