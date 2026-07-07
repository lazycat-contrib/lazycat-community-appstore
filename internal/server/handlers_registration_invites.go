package server

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/registrationinvite"
)

const maxInviteUses = 10000

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
	records, err := s.db.RegistrationInvite.Query().
		Order(entgo.Desc(registrationinvite.FieldCreatedAt)).
		Limit(200).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INVITE_LIST_FAILED", "Could not list registration invites", nil)
		return
	}
	invites := make([]registrationInviteDTO, 0, len(records))
	for _, record := range records {
		invites = append(invites, toRegistrationInviteDTO(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": invites})
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
	code, err := randomInviteCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INVITE_CREATE_FAILED", "Could not create registration invite", nil)
		return
	}
	record, err := s.db.RegistrationInvite.Create().
		SetCode(code).
		SetCodeHash(tokenHash(code)).
		SetCodePrefix(tokenPrefix(code)).
		SetNote(input.Note).
		SetMaxUses(input.MaxUses).
		SetRemainingUses(input.MaxUses).
		SetCreatedBy(u.ID).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INVITE_CREATE_FAILED", "Could not create registration invite", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invite": toRegistrationInviteDTO(record), "code": code})
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
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "lciv_" + base64.RawURLEncoding.EncodeToString(buf[:]), nil
}
