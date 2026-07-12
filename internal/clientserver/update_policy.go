package clientserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"lazycat.community/appstore/ent/clientappupdatepolicy"
)

func normalizePolicyPackageID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *Server) effectiveAutoUpdatePolicies(ctx context.Context, userID string, packageIDs []string) (map[string]bool, error) {
	effective := make(map[string]bool, len(packageIDs))
	normalized := make([]string, 0, len(packageIDs))
	seen := make(map[string]struct{}, len(packageIDs))
	for _, packageID := range packageIDs {
		packageID = normalizePolicyPackageID(packageID)
		if packageID == "" {
			continue
		}
		effective[packageID] = true
		if _, ok := seen[packageID]; ok {
			continue
		}
		seen[packageID] = struct{}{}
		normalized = append(normalized, packageID)
	}
	if len(normalized) == 0 {
		return effective, nil
	}
	records, err := s.db.ClientAppUpdatePolicy.Query().
		Where(
			clientappupdatepolicy.UserIDEQ(userID),
			clientappupdatepolicy.PackageIDIn(normalized...),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		effective[record.PackageID] = record.AutoUpdateEnabled
	}
	return effective, nil
}

func (s *Server) setAutoUpdatePolicy(ctx context.Context, userID, packageID string, enabled bool) (ClientAppUpdatePolicyDTO, error) {
	packageID = normalizePolicyPackageID(packageID)
	if packageID == "" {
		return ClientAppUpdatePolicyDTO{}, errors.New("package ID is required")
	}
	updated, err := s.db.ClientAppUpdatePolicy.Update().
		Where(
			clientappupdatepolicy.UserIDEQ(userID),
			clientappupdatepolicy.PackageIDEQ(packageID),
		).
		SetAutoUpdateEnabled(enabled).
		Save(ctx)
	if err != nil {
		return ClientAppUpdatePolicyDTO{}, err
	}
	if updated == 0 {
		_, err = s.db.ClientAppUpdatePolicy.Create().
			SetUserID(userID).
			SetPackageID(packageID).
			SetAutoUpdateEnabled(enabled).
			Save(ctx)
		if err != nil {
			return ClientAppUpdatePolicyDTO{}, err
		}
	}
	return ClientAppUpdatePolicyDTO{PackageID: packageID, AutoUpdateEnabled: enabled}, nil
}

func (s *Server) disabledAutoUpdatePackageIDs(ctx context.Context, userID string) (map[string]struct{}, error) {
	records, err := s.db.ClientAppUpdatePolicy.Query().
		Where(
			clientappupdatepolicy.UserIDEQ(userID),
			clientappupdatepolicy.AutoUpdateEnabledEQ(false),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	disabled := make(map[string]struct{}, len(records))
	for _, record := range records {
		disabled[record.PackageID] = struct{}{}
	}
	return disabled, nil
}

func (s *Server) handleSetAutoUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	packageID := normalizePolicyPackageID(r.PathValue("packageId"))
	if packageID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_PACKAGE_ID", "Package ID is required")
		return
	}
	var input ClientAppUpdatePolicyInput
	if err := decodeJSON(r, &input); err != nil || input.AutoUpdateEnabled == nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body")
		return
	}
	policy, err := s.setAutoUpdatePolicy(r.Context(), currentUserID(r), packageID, *input.AutoUpdateEnabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_POLICY_SAVE_FAILED", "Could not save automatic update policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}
