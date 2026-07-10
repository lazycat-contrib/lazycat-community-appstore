package server

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/registrationinvite"
	"lazycat.community/appstore/internal/pagination"
)

const (
	maxInviteUses       = 10000
	inviteCodeLength    = 8
	inviteCodeAlphabet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	inviteCreateRetries = 5
)

type registrationInviteDTO struct {
	ID            int       `json:"id"`
	Code          string    `json:"code"`
	CodePrefix    string    `json:"codePrefix"`
	Note          string    `json:"note"`
	MaxUses       int       `json:"maxUses"`
	RemainingUses int       `json:"remainingUses"`
	CreatedBy     int       `json:"createdBy"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type createRegistrationInviteRequest struct {
	Note    string `json:"note"`
	MaxUses int    `json:"maxUses"`
}

func (s *Server) handleListRegistrationInvites(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	page := pagination.FromRequest(r, s.effectiveDefaultPageSize(r.Context(), 50, 200), 200)
	q := s.db.RegistrationInvite.Query()
	total, err := q.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INVITE_LIST_FAILED", "Could not list registration invites", nil)
		return
	}
	records, err := q.
		Order(entgo.Desc(registrationinvite.FieldCreatedAt)).
		Offset(page.Offset()).
		Limit(page.PageSize).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INVITE_LIST_FAILED", "Could not list registration invites", nil)
		return
	}
	invites := make([]registrationInviteDTO, 0, len(records))
	for _, record := range records {
		invites = append(invites, toRegistrationInviteDTO(record))
	}
	writeJSON(w, http.StatusOK, pagination.NewInvitesPage(invites, page, total))
}

func (s *Server) handleCreateRegistrationInvite(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input createRegistrationInviteRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if len([]rune(input.Note)) > 160 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invite note must be 160 characters or fewer", nil)
		return
	}
	if input.MaxUses == 0 {
		input.MaxUses = 1
	}
	if input.MaxUses < 1 || input.MaxUses > maxInviteUses {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invite uses must be between 1 and 10000", nil)
		return
	}
	var code string
	var record *entgo.RegistrationInvite
	var err error
	for range inviteCreateRetries {
		code, err = randomInviteCode()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INVITE_CREATE_FAILED", "Could not create registration invite", nil)
			return
		}
		record, err = s.db.RegistrationInvite.Create().
			SetCode(code).
			SetCodeHash(tokenHash(code)).
			SetCodePrefix(tokenPrefix(code)).
			SetNote(input.Note).
			SetMaxUses(input.MaxUses).
			SetRemainingUses(input.MaxUses).
			SetCreatedBy(u.ID).
			Save(r.Context())
		if err == nil {
			writeJSON(w, http.StatusCreated, map[string]any{"invite": toRegistrationInviteDTO(record), "code": code})
			return
		}
		if !entgo.IsConstraintError(err) {
			break
		}
	}
	writeError(w, http.StatusInternalServerError, "INVITE_CREATE_FAILED", "Could not create registration invite", nil)
}

func (s *Server) handleDeleteRegistrationInvite(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.db.RegistrationInvite.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusNotFound, "INVITE_NOT_FOUND", "Registration invite not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func toRegistrationInviteDTO(record *entgo.RegistrationInvite) registrationInviteDTO {
	return registrationInviteDTO{
		ID:            record.ID,
		Code:          record.Code,
		CodePrefix:    record.CodePrefix,
		Note:          record.Note,
		MaxUses:       record.MaxUses,
		RemainingUses: record.RemainingUses,
		CreatedBy:     record.CreatedBy,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}

func randomInviteCode() (string, error) {
	var code [inviteCodeLength]byte
	max := big.NewInt(int64(len(inviteCodeAlphabet)))
	for i := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		code[i] = inviteCodeAlphabet[n.Int64()]
	}
	return string(code[:]), nil
}
