package pagination

import (
	"net/http"
	"strconv"
)

type Params struct {
	Page     int
	PageSize int
}

type Response struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

type Page[T any] struct {
	Items      []T      `json:"items"`
	Pagination Response `json:"pagination"`
}

type AppsPage[T any] struct {
	Apps       []T      `json:"apps"`
	Pagination Response `json:"pagination"`
}

func FromRequest(r *http.Request, defaultPageSize, maxPageSize int) Params {
	if defaultPageSize <= 0 {
		defaultPageSize = 24
	}
	if maxPageSize < defaultPageSize {
		maxPageSize = defaultPageSize
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return Params{Page: page, PageSize: pageSize}
}

func (p Params) Offset() int {
	if p.Page <= 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

func (p Params) Response(totalItems int) Response {
	totalPages := 0
	if totalItems > 0 && p.PageSize > 0 {
		totalPages = (totalItems + p.PageSize - 1) / p.PageSize
	}
	return Response{
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

func NewPage[T any](items []T, params Params, totalItems int) Page[T] {
	return Page[T]{
		Items:      items,
		Pagination: params.Response(totalItems),
	}
}

func NewAppsPage[T any](items []T, params Params, totalItems int) AppsPage[T] {
	return AppsPage[T]{
		Apps:       items,
		Pagination: params.Response(totalItems),
	}
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
