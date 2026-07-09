package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/collection"
	"lazycat.community/appstore/ent/collectionapp"
)

type collectionRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	AppIDs      []int  `json:"appIds"`
}

func (s *Server) handleAdminListCollections(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.listCollections(w, r, u, true)
}

func (s *Server) handlePublicListCollections(w http.ResponseWriter, r *http.Request) {
	u := s.optionalUser(r)
	if u != nil {
		s.listCollections(w, r, u, false)
		return
	}
	value, err := s.sharedFirstLoad(r.Context(), firstLoadKey(r, "public-collections"), func(ctx context.Context) (any, error) {
		return s.publicListCollectionsResponse(ctx, r)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COLLECTION_LIST_FAILED", "Could not list collections", nil)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) listCollections(w http.ResponseWriter, r *http.Request, u *entgo.User, includeDrafts bool) {
	records, err := s.db.Collection.Query().Order(entgo.Asc(collection.FieldSortOrder), entgo.Asc(collection.FieldName)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COLLECTION_LIST_FAILED", "Could not list collections", nil)
		return
	}
	out := make([]collectionDTO, 0, len(records))
	for _, record := range records {
		out = append(out, s.buildCollectionDTO(r, record, u, includeDrafts))
	}
	writeJSON(w, http.StatusOK, map[string]any{"collections": out})
}

func (s *Server) publicListCollectionsResponse(ctx context.Context, r *http.Request) (any, error) {
	req := r.Clone(ctx)
	records, err := s.db.Collection.Query().Order(entgo.Asc(collection.FieldSortOrder), entgo.Asc(collection.FieldName)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]collectionDTO, 0, len(records))
	for _, record := range records {
		out = append(out, s.buildCollectionDTO(req, record, nil, false))
	}
	return map[string]any{"collections": out}, nil
}

func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input collectionRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Collection name is required", nil)
		return
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	kind := collection.Kind(strings.ToUpper(strings.TrimSpace(input.Kind)))
	if kind == "" {
		kind = collection.KindMANUAL
	}
	if err := collection.KindValidator(kind); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid collection kind", nil)
		return
	}
	record, err := s.db.Collection.Create().
		SetCreatorID(u.ID).
		SetName(name).
		SetSlug(slug).
		SetDescription(input.Description).
		SetKind(kind).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "COLLECTION_CREATE_FAILED", "Could not create collection", nil)
		return
	}
	_ = s.syncCollectionApps(r, record.ID, input.AppIDs)
	writeJSON(w, http.StatusCreated, map[string]any{"collection": s.buildCollectionDTO(r, record, u, true)})
}

func (s *Server) handleUpdateCollection(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	var input collectionRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	update := s.db.Collection.UpdateOneID(id)
	if name := strings.TrimSpace(input.Name); name != "" {
		update.SetName(name)
	}
	if slug := strings.TrimSpace(input.Slug); slug != "" {
		update.SetSlug(slug)
	}
	update.SetDescription(input.Description)
	if kindRaw := strings.TrimSpace(input.Kind); kindRaw != "" {
		kind := collection.Kind(strings.ToUpper(kindRaw))
		if err := collection.KindValidator(kind); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid collection kind", nil)
			return
		}
		update.SetKind(kind)
	}
	record, err := update.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "COLLECTION_UPDATE_FAILED", "Could not update collection", nil)
		return
	}
	_ = s.syncCollectionApps(r, record.ID, input.AppIDs)
	writeJSON(w, http.StatusOK, map[string]any{"collection": s.buildCollectionDTO(r, record, u, true)})
}

func (s *Server) handleDeleteCollection(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	_, _ = s.db.CollectionApp.Delete().Where(collectionapp.CollectionIDEQ(id)).Exec(r.Context())
	if err := s.db.Collection.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "COLLECTION_DELETE_FAILED", "Could not delete collection", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) syncCollectionApps(r *http.Request, collectionID int, appIDs []int) error {
	_, _ = s.db.CollectionApp.Delete().Where(collectionapp.CollectionIDEQ(collectionID)).Exec(r.Context())
	for order, appID := range appIDs {
		if appID <= 0 {
			continue
		}
		_, _ = s.db.CollectionApp.Create().SetCollectionID(collectionID).SetAppID(appID).SetSortOrder(order).Save(r.Context())
	}
	return nil
}

func (s *Server) buildCollectionDTO(r *http.Request, record *entgo.Collection, u *entgo.User, includeDrafts bool) collectionDTO {
	dto := collectionDTO{
		ID:          record.ID,
		Name:        record.Name,
		Slug:        record.Slug,
		Description: record.Description,
		Kind:        string(record.Kind),
		Apps:        []appSummary{},
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
	var records []*entgo.App
	switch record.Kind {
	case collection.KindRECENT_UPDATED:
		records, _ = s.db.App.Query().Where(app.StatusEQ(app.StatusAPPROVED)).Order(entgo.Desc(app.FieldUpdatedAt)).Limit(12).All(r.Context())
	case collection.KindMOST_DOWNLOADED:
		records, _ = s.db.App.Query().Where(app.StatusEQ(app.StatusAPPROVED)).Order(entgo.Desc(app.FieldDownloadCount)).Limit(12).All(r.Context())
	default:
		links, _ := s.db.CollectionApp.Query().Where(collectionapp.CollectionIDEQ(record.ID)).Order(entgo.Asc(collectionapp.FieldSortOrder)).All(r.Context())
		for _, link := range links {
			appRecord, err := s.db.App.Get(r.Context(), link.AppID)
			if err == nil {
				records = append(records, appRecord)
			}
		}
	}
	for _, appRecord := range records {
		if !includeDrafts && appRecord.Status != app.StatusAPPROVED {
			continue
		}
		if !s.userCanSeeApp(r, appRecord, u) {
			continue
		}
		dto.Apps = append(dto.Apps, s.appSummaryDTO(r, appRecord, u))
	}
	return dto
}
