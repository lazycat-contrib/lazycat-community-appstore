package pagination

import (
	"net/http"
	"strconv"
)

const DefaultPageSize = 24

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

type ReviewsPage[T any] struct {
	Reviews    []T      `json:"reviews"`
	Pagination Response `json:"pagination"`
}

type UsersPage[T any] struct {
	Users      []T      `json:"users"`
	Pagination Response `json:"pagination"`
}

type InvitesPage[T any] struct {
	Invites    []T      `json:"invites"`
	Pagination Response `json:"pagination"`
}

type HistoryPage[T any] struct {
	History    []T      `json:"history"`
	Pagination Response `json:"pagination"`
}

type NotificationsPage[T any] struct {
	Notifications []T      `json:"notifications"`
	Pagination    Response `json:"pagination"`
}

type FavoritesPage[TApp, TSubmitter any] struct {
	Apps       []TApp       `json:"apps"`
	Submitters []TSubmitter `json:"submitters"`
	Pagination Response     `json:"pagination"`
}

type Favorites[TApp, TSubmitter any] struct {
	Apps       []TApp       `json:"apps"`
	Submitters []TSubmitter `json:"submitters"`
}

func FromRequest(r *http.Request, defaultPageSize, maxPageSize int) Params {
	defaultPageSize = ClampPageSize(defaultPageSize, DefaultPageSize, maxPageSize)
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return Params{Page: page, PageSize: pageSize}
}

func ClampPageSize(value, fallback, maxPageSize int) int {
	if fallback <= 0 {
		fallback = DefaultPageSize
	}
	if maxPageSize < fallback {
		maxPageSize = fallback
	}
	if value <= 0 {
		return fallback
	}
	if value > maxPageSize {
		return maxPageSize
	}
	return value
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

func NewReviewsPage[T any](items []T, params Params, totalItems int) ReviewsPage[T] {
	return ReviewsPage[T]{
		Reviews:    items,
		Pagination: params.Response(totalItems),
	}
}

func NewUsersPage[T any](items []T, params Params, totalItems int) UsersPage[T] {
	return UsersPage[T]{
		Users:      items,
		Pagination: params.Response(totalItems),
	}
}

func NewInvitesPage[T any](items []T, params Params, totalItems int) InvitesPage[T] {
	return InvitesPage[T]{
		Invites:    items,
		Pagination: params.Response(totalItems),
	}
}

func NewHistoryPage[T any](items []T, params Params, totalItems int) HistoryPage[T] {
	return HistoryPage[T]{
		History:    items,
		Pagination: params.Response(totalItems),
	}
}

func NewNotificationsPage[T any](items []T, params Params, totalItems int) NotificationsPage[T] {
	return NotificationsPage[T]{
		Notifications: items,
		Pagination:    params.Response(totalItems),
	}
}

func NewFavoritesPage[TApp, TSubmitter any](apps []TApp, submitters []TSubmitter, params Params, totalItems int) FavoritesPage[TApp, TSubmitter] {
	return FavoritesPage[TApp, TSubmitter]{
		Apps:       apps,
		Submitters: submitters,
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
