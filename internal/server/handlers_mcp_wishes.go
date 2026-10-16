package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/wish"
	"lazycat.community/appstore/ent/wishreply"
	"lazycat.community/appstore/ent/wishstatusevent"
	"lazycat.community/appstore/internal/pagination"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpListWishesInput struct {
	Kind     wish.Kind   `json:"kind,omitempty" jsonschema:"filter by SUGGESTION, APP_REQUEST, or CUSTOMIZATION"`
	Status   wish.Status `json:"status,omitempty" jsonschema:"filter by OPEN, PLANNED, IN_PROGRESS, COMPLETED, or REJECTED"`
	Query    string      `json:"query,omitempty" jsonschema:"search wish titles and descriptions"`
	MineOnly bool        `json:"mineOnly,omitempty" jsonschema:"show only wishes created by this MCP user"`
	Page     int         `json:"page,omitempty" jsonschema:"page number, starting at 1"`
	Limit    int         `json:"limit,omitempty" jsonschema:"items per page, up to 100"`
}

type mcpWishesOutput struct {
	Items      []wishDTO           `json:"items"`
	Pagination pagination.Response `json:"pagination"`
	NextPage   int                 `json:"nextPage,omitempty"`
}

type mcpWishIDInput struct {
	ID int `json:"id" jsonschema:"wish ID"`
}

type mcpWishOutput struct {
	Wish wishDTO `json:"wish"`
}

type mcpUpdateWishInput struct {
	ID           int     `json:"id" jsonschema:"wish ID"`
	Title        *string `json:"title,omitempty" jsonschema:"replacement title; omit to preserve"`
	Body         *string `json:"body,omitempty" jsonschema:"replacement description; omit to preserve"`
	ReferenceURL *string `json:"referenceUrl,omitempty" jsonschema:"replacement reference URL; empty string clears it"`
	ContactEmail *string `json:"contactEmail,omitempty" jsonschema:"replacement customization contact email"`
	ContactOther *string `json:"contactOther,omitempty" jsonschema:"replacement other contact information"`
}

type mcpWishReplyInput struct {
	ID   int    `json:"id" jsonschema:"wish ID"`
	Body string `json:"body" jsonschema:"reply text, 1 to 5000 characters"`
}

type mcpWishReplyOutput struct {
	Reply wishReplyDTO `json:"reply"`
}

type mcpWishStatusInput struct {
	ID         int         `json:"id" jsonschema:"wish ID"`
	Status     wish.Status `json:"status" jsonschema:"new status"`
	StatusText string      `json:"statusText" jsonschema:"required explanation, saved in immutable history"`
}

type mcpWishStatusOutput struct {
	StatusEvent wishStatusEventDTO `json:"statusEvent"`
}

func (s *Server) registerWishMCPTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_list_wishes", Title: "List wishes",
		Description: "List public wishes and your own private suggestions. Admin MCP tokens can see all wishes. Supports filters and pagination."}, s.mcpListWishes)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_get_wish", Title: "Get a wish",
		Description: "Read a public wish or one of your own private suggestions. Admin MCP tokens can read any wish."}, s.mcpGetWish)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_create_wish", Title: "Create a wish",
		Description: "Create your own suggestion, app request, or customization wish. statusText is required for the initial OPEN history entry."}, s.mcpCreateWish)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_update_wish", Title: "Update a wish",
		Description: "Edit your own wish. Admin MCP tokens may also edit another person's wish. Omitted fields are preserved."}, s.mcpUpdateWish)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_delete_wish", Title: "Delete a wish",
		Description: "Delete your own wish and its history. Admin MCP tokens may also delete another person's wish."}, s.mcpDeleteWish)
}

func (s *Server) registerAdminWishMCPTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_reply_to_wish", Title: "Reply to a wish",
		Description: "Admin-only reply to any wish as the token user."}, s.mcpAdminReplyToWish)
	mcp.AddTool(server, &mcp.Tool{Name: "appstore_admin_set_wish_status", Title: "Set wish status",
		Description: "Admin-only status change for any wish. A nonempty explanation is required and added to status history."}, s.mcpAdminSetWishStatus)
}

func mcpWishOwnerID(userID int) string {
	return fmt.Sprintf("mcp_user_%d", userID)
}

func mcpWishViewer(u *entgo.User, principalType mcptoken.PrincipalType) wishViewer {
	if principalType == mcptoken.PrincipalTypeADMIN {
		return wishViewer{User: u, OwnerID: mcpWishOwnerID(u.ID)}
	}
	return wishViewer{OwnerID: mcpWishOwnerID(u.ID)}
}

func (s *Server) mcpWishDTO(ctx context.Context, record *entgo.Wish, viewer wishViewer) (wishDTO, error) {
	dto, err := s.wishToDTO(ctx, record, viewer)
	if err == nil && viewer.isAdmin() {
		dto.CanManage = true
	}
	return dto, err
}

func (s *Server) mcpListWishes(ctx context.Context, req *mcp.CallToolRequest, input mcpListWishesInput) (*mcp.CallToolResult, mcpWishesOutput, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpWishesOutput{}, err
	}
	if input.Page < 0 {
		return nil, mcpWishesOutput{}, errors.New("page must be at least 1")
	}
	page := max(1, input.Page)
	pageSize := clampLimit(input.Limit, 24, 100)
	if page-1 > math.MaxInt/pageSize {
		return nil, mcpWishesOutput{}, errors.New("page is too large")
	}
	ownerID := mcpWishOwnerID(u.ID)
	query := s.db.Wish.Query()
	if input.MineOnly {
		query.Where(wish.OwnerIDEQ(ownerID))
	} else if principalType != mcptoken.PrincipalTypeADMIN {
		query.Where(wish.Or(wish.KindIn(wish.KindAPP_REQUEST, wish.KindCUSTOMIZATION), wish.OwnerIDEQ(ownerID)))
	}
	if input.Kind != "" {
		if wish.KindValidator(input.Kind) != nil {
			return nil, mcpWishesOutput{}, errors.New("wish kind is invalid")
		}
		query.Where(wish.KindEQ(input.Kind))
	}
	if input.Status != "" {
		if wish.StatusValidator(input.Status) != nil {
			return nil, mcpWishesOutput{}, errors.New("wish status is invalid")
		}
		query.Where(wish.StatusEQ(input.Status))
	}
	if search := strings.TrimSpace(input.Query); search != "" {
		query.Where(wish.Or(wish.TitleContainsFold(search), wish.BodyContainsFold(search)))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, mcpWishesOutput{}, err
	}
	params := pagination.Params{Page: page, PageSize: pageSize}
	records, err := query.Order(entgo.Desc(wish.FieldLastActivityAt), entgo.Desc(wish.FieldID)).
		Offset(params.Offset()).Limit(pageSize).All(ctx)
	if err != nil {
		return nil, mcpWishesOutput{}, err
	}
	viewer := mcpWishViewer(u, principalType)
	out := mcpWishesOutput{Items: make([]wishDTO, 0, len(records)), Pagination: params.Response(total)}
	for _, record := range records {
		dto, err := s.mcpWishDTO(ctx, record, viewer)
		if err != nil {
			return nil, mcpWishesOutput{}, err
		}
		out.Items = append(out.Items, dto)
	}
	if page < out.Pagination.TotalPages {
		out.NextPage = page + 1
	}
	return nil, out, nil
}

func (s *Server) mcpGetWish(ctx context.Context, req *mcp.CallToolRequest, input mcpWishIDInput) (*mcp.CallToolResult, mcpWishOutput, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpWishOutput{}, err
	}
	record, err := s.mcpReadableWish(ctx, input.ID, mcpWishOwnerID(u.ID), principalType)
	if err != nil {
		return nil, mcpWishOutput{}, err
	}
	dto, err := s.mcpWishDTO(ctx, record, mcpWishViewer(u, principalType))
	return nil, mcpWishOutput{Wish: dto}, err
}

func (s *Server) mcpReadableWish(ctx context.Context, id int, ownerID string, principalType mcptoken.PrincipalType) (*entgo.Wish, error) {
	if id <= 0 {
		return nil, errors.New("wish ID is required")
	}
	record, err := s.db.Wish.Get(ctx, id)
	if err != nil {
		return nil, errors.New("wish not found")
	}
	if record.Kind == wish.KindSUGGESTION && principalType != mcptoken.PrincipalTypeADMIN && record.OwnerID != ownerID {
		return nil, errors.New("wish not found")
	}
	return record, nil
}

func (s *Server) mcpManagedWish(ctx context.Context, id int, ownerID string, principalType mcptoken.PrincipalType) (*entgo.Wish, error) {
	record, err := s.mcpReadableWish(ctx, id, ownerID, principalType)
	if err != nil {
		return nil, err
	}
	if principalType != mcptoken.PrincipalTypeADMIN && record.OwnerID != ownerID {
		return nil, errors.New("wish not found")
	}
	return record, nil
}

func (s *Server) mcpCreateWish(ctx context.Context, req *mcp.CallToolRequest, input createWishInput) (*mcp.CallToolResult, mcpWishOutput, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpWishOutput{}, err
	}
	if message := validateWishInput(&input); message != "" {
		return nil, mcpWishOutput{}, errors.New(message)
	}
	ownerID := mcpWishOwnerID(u.ID)
	record, err := s.createWish(ctx, wishActor{ClientUserID: ownerID, OwnerID: ownerID, DisplayName: userDisplayName(u), UserID: u.ID}, input)
	if err != nil {
		return nil, mcpWishOutput{}, fmt.Errorf("create wish: %w", err)
	}
	dto, err := s.mcpWishDTO(ctx, record, mcpWishViewer(u, principalType))
	return nil, mcpWishOutput{Wish: dto}, err
}

func (s *Server) mcpUpdateWish(ctx context.Context, req *mcp.CallToolRequest, input mcpUpdateWishInput) (*mcp.CallToolResult, mcpWishOutput, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpWishOutput{}, err
	}
	record, err := s.mcpManagedWish(ctx, input.ID, mcpWishOwnerID(u.ID), principalType)
	if err != nil {
		return nil, mcpWishOutput{}, err
	}
	if input.Title == nil && input.Body == nil && input.ReferenceURL == nil && input.ContactEmail == nil && input.ContactOther == nil {
		return nil, mcpWishOutput{}, errors.New("provide at least one field to update")
	}
	validated := createWishInput{Kind: record.Kind, Title: record.Title, Body: record.Body,
		ReferenceURL: record.ReferenceURL, ContactEmail: record.ContactEmail, ContactOther: record.ContactOther, StatusText: "unchanged"}
	if input.Title != nil {
		validated.Title = *input.Title
	}
	if input.Body != nil {
		validated.Body = *input.Body
	}
	if input.ReferenceURL != nil {
		validated.ReferenceURL = *input.ReferenceURL
	}
	if input.ContactEmail != nil {
		validated.ContactEmail = *input.ContactEmail
	}
	if input.ContactOther != nil {
		validated.ContactOther = *input.ContactOther
	}
	if message := validateWishInput(&validated); message != "" {
		return nil, mcpWishOutput{}, errors.New(message)
	}
	now := s.currentTime().UTC()
	record, err = s.db.Wish.UpdateOne(record).SetTitle(validated.Title).SetBody(validated.Body).
		SetReferenceURL(validated.ReferenceURL).SetContactEmail(validated.ContactEmail).SetContactOther(validated.ContactOther).
		SetUpdatedAt(now).SetLastActivityAt(now).Save(ctx)
	if err != nil {
		return nil, mcpWishOutput{}, fmt.Errorf("update wish: %w", err)
	}
	dto, err := s.mcpWishDTO(ctx, record, mcpWishViewer(u, principalType))
	return nil, mcpWishOutput{Wish: dto}, err
}

func (s *Server) mcpDeleteWish(ctx context.Context, req *mcp.CallToolRequest, input mcpWishIDInput) (*mcp.CallToolResult, okResponse, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, okResponse{}, err
	}
	if _, err := s.mcpManagedWish(ctx, input.ID, mcpWishOwnerID(u.ID), principalType); err != nil {
		return nil, okResponse{}, err
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, okResponse{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.WishReply.Delete().Where(wishreply.WishIDEQ(input.ID)).Exec(ctx); err == nil {
		_, err = tx.WishStatusEvent.Delete().Where(wishstatusevent.WishIDEQ(input.ID)).Exec(ctx)
	}
	if err == nil {
		err = tx.Wish.DeleteOneID(input.ID).Exec(ctx)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return nil, okResponse{}, fmt.Errorf("delete wish: %w", err)
	}
	return nil, okResponse{OK: true}, nil
}

func (s *Server) mcpAdminReplyToWish(ctx context.Context, req *mcp.CallToolRequest, input mcpWishReplyInput) (*mcp.CallToolResult, mcpWishReplyOutput, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpWishReplyOutput{}, err
	}
	if principalType != mcptoken.PrincipalTypeADMIN {
		return nil, mcpWishReplyOutput{}, errors.New("admin MCP token is required")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" || utf8.RuneCountInString(body) > 5000 {
		return nil, mcpWishReplyOutput{}, errors.New("reply must be between 1 and 5000 characters")
	}
	if _, err := s.mcpReadableWish(ctx, input.ID, "", principalType); err != nil {
		return nil, mcpWishReplyOutput{}, err
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, mcpWishReplyOutput{}, err
	}
	defer func() { _ = tx.Rollback() }()
	now := s.currentTime().UTC()
	reply, err := tx.WishReply.Create().SetWishID(input.ID).SetAuthorUserID(u.ID).
		SetAuthorName(userDisplayName(u)).SetBody(body).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
	if err == nil {
		_, err = tx.Wish.UpdateOneID(input.ID).SetLastActivityAt(now).SetUpdatedAt(now).Save(ctx)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return nil, mcpWishReplyOutput{}, fmt.Errorf("reply to wish: %w", err)
	}
	return nil, mcpWishReplyOutput{Reply: wishReplyDTO{ID: reply.ID, AuthorName: reply.AuthorName, Body: reply.Body,
		CreatedAt: reply.CreatedAt, UpdatedAt: reply.UpdatedAt}}, nil
}

func (s *Server) mcpAdminSetWishStatus(ctx context.Context, req *mcp.CallToolRequest, input mcpWishStatusInput) (*mcp.CallToolResult, mcpWishStatusOutput, error) {
	u, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return nil, mcpWishStatusOutput{}, err
	}
	if principalType != mcptoken.PrincipalTypeADMIN {
		return nil, mcpWishStatusOutput{}, errors.New("admin MCP token is required")
	}
	input.StatusText = strings.TrimSpace(input.StatusText)
	if wish.StatusValidator(input.Status) != nil || input.StatusText == "" || utf8.RuneCountInString(input.StatusText) > 1000 {
		return nil, mcpWishStatusOutput{}, errors.New("a valid status and status text are required")
	}
	record, err := s.mcpReadableWish(ctx, input.ID, "", principalType)
	if err != nil {
		return nil, mcpWishStatusOutput{}, err
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, mcpWishStatusOutput{}, err
	}
	defer func() { _ = tx.Rollback() }()
	now := s.currentTime().UTC()
	event, err := tx.WishStatusEvent.Create().SetWishID(input.ID).SetFromStatus(record.Status.String()).
		SetToStatus(wishstatusevent.ToStatus(input.Status)).SetActorType(wishstatusevent.ActorTypeUSER).
		SetActorUserID(u.ID).SetActorName(userDisplayName(u)).SetText(input.StatusText).SetCreatedAt(now).Save(ctx)
	if err == nil {
		_, err = tx.Wish.UpdateOneID(input.ID).SetStatus(input.Status).SetLastActivityAt(now).SetUpdatedAt(now).Save(ctx)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return nil, mcpWishStatusOutput{}, fmt.Errorf("set wish status: %w", err)
	}
	return nil, mcpWishStatusOutput{StatusEvent: wishStatusEventDTO{ID: event.ID, From: event.FromStatus, To: event.ToStatus,
		ActorType: event.ActorType, ActorName: event.ActorName, Text: event.Text, CreatedAt: event.CreatedAt}}, nil
}
