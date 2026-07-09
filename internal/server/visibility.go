package server

import (
	"context"
	"net/http"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/groupmember"
)

func (s *Server) appIsPublic(ctx context.Context, appID int) bool {
	count, err := s.db.AppVisibility.Query().Where(appvisibility.AppIDEQ(appID)).Count(ctx)
	return err == nil && count == 0
}

func (s *Server) visibleGroupIDs(ctx context.Context, appID int) []int {
	records, err := s.db.AppVisibility.Query().Where(appvisibility.AppIDEQ(appID)).All(ctx)
	if err != nil {
		return nil
	}
	ids := make([]int, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.GroupID)
	}
	return ids
}

func (s *Server) userCanSeeApp(r *http.Request, record *entgo.App, u *entgo.User) bool {
	if s.appIsPublic(r.Context(), record.ID) {
		return true
	}
	if u == nil {
		return false
	}
	if isAdmin(u) || record.OwnerID == u.ID || s.isCollaborator(r, record.ID, u.ID) {
		return true
	}
	groupIDs := s.visibleGroupIDs(r.Context(), record.ID)
	if len(groupIDs) == 0 {
		return true
	}
	ok, err := s.db.GroupMember.Query().
		Where(groupmember.UserIDEQ(u.ID), groupmember.GroupIDIn(groupIDs...)).
		Exist(r.Context())
	return err == nil && ok
}

func userCanSeeAppFromPreload(record *entgo.App, u *entgo.User, preload appSummaryPreload, userGroupIDs map[int]struct{}) bool {
	groupIDs := preload.visibleGroupIDs[record.ID]
	if len(groupIDs) == 0 {
		return true
	}
	if u == nil {
		return false
	}
	if isAdmin(u) || record.OwnerID == u.ID {
		return true
	}
	if _, ok := preload.collaboratorAppIDs[record.ID]; ok {
		return true
	}
	for _, groupID := range groupIDs {
		if _, ok := userGroupIDs[groupID]; ok {
			return true
		}
	}
	return false
}
