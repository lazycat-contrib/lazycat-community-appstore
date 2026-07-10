package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/usergroup"
)

const groupCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var errGroupCodeCollision = errors.New("group code collision")

type groupDTO struct {
	ID               int       `json:"id"`
	OwnerID          int       `json:"ownerId"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	Description      string    `json:"description"`
	Code             string    `json:"code"`
	CodeUpdatedAt    time.Time `json:"codeUpdatedAt"`
	MemberCount      int       `json:"memberCount"`
	AttachedAppCount int       `json:"attachedAppCount"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type sourceGroupDTO struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name"`
	Code string `json:"code,omitempty"`
}

type groupCodeAccess struct {
	validGroupIDs     []int
	validGroups       []sourceGroupDTO
	invalidGroupCodes []string
	codeByGroupID     map[int]string
}

type groupClientConfig struct {
	SourceURL  string           `json:"sourceUrl"`
	GroupCodes []string         `json:"groupCodes"`
	Groups     []sourceGroupDTO `json:"groups"`
}

func generateGroupCode() string {
	var b [6]byte
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(groupCodeAlphabet))))
		if err != nil {
			panic(err)
		}
		b[i] = groupCodeAlphabet[n.Int64()]
	}
	return string(b[:])
}

func normalizeGroupCode(value string) string {
	code := strings.ToUpper(strings.TrimSpace(value))
	if len(code) != 6 {
		return ""
	}
	for _, r := range code {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return code
}

func normalizeGroupCodes(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		code := normalizeGroupCode(value)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func groupCodesFromRequest(r *http.Request) []string {
	parts := []string{}
	if value := r.URL.Query().Get("groupCodes"); value != "" {
		parts = append(parts, splitGroupCodeList(value)...)
	}
	if value := r.Header.Get("X-Group-Codes"); value != "" {
		parts = append(parts, splitGroupCodeList(value)...)
	}
	return normalizeGroupCodes(parts)
}

func splitGroupCodeList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
}

func (s *Server) resolveGroupCodeAccess(ctx context.Context, codes []string) (groupCodeAccess, error) {
	codes = normalizeGroupCodes(codes)
	access := groupCodeAccess{codeByGroupID: map[int]string{}}
	if len(codes) == 0 {
		return access, nil
	}
	records, err := s.db.UserGroup.Query().
		Where(usergroup.CodeIn(codes...)).
		All(ctx)
	if err != nil {
		return access, err
	}
	byCode := make(map[string]*entgo.UserGroup, len(records))
	for _, record := range records {
		if record.Code != "" {
			byCode[record.Code] = record
		}
	}
	for _, code := range codes {
		record := byCode[code]
		if record == nil {
			access.invalidGroupCodes = append(access.invalidGroupCodes, code)
			continue
		}
		access.validGroupIDs = append(access.validGroupIDs, record.ID)
		access.validGroups = append(access.validGroups, sourceGroupDTO{ID: record.ID, Name: record.Name, Code: record.Code})
		access.codeByGroupID[record.ID] = code
	}
	return access, nil
}

func (s *Server) ensureGroupCode(ctx context.Context, record *entgo.UserGroup) (*entgo.UserGroup, error) {
	if normalizeGroupCode(record.Code) != "" {
		return record, nil
	}
	return s.rotateGroupCode(ctx, record.ID)
}

func (s *Server) rotateGroupCode(ctx context.Context, groupID int) (*entgo.UserGroup, error) {
	for range 8 {
		code := generateGroupCode()
		exists, err := s.db.UserGroup.Query().
			Where(usergroup.CodeEQ(code), usergroup.IDNEQ(groupID)).
			Exist(ctx)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		now := time.Now()
		return s.db.UserGroup.UpdateOneID(groupID).
			SetCode(code).
			SetCodeUpdatedAt(now).
			Save(ctx)
	}
	return nil, errGroupCodeCollision
}

func (s *Server) createGroupCode(ctx context.Context) (string, error) {
	for range 8 {
		code := generateGroupCode()
		exists, err := s.db.UserGroup.Query().Where(usergroup.CodeEQ(code)).Exist(ctx)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errGroupCodeCollision
}

func encodeGroupClientConfig(config groupClientConfig) (string, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
