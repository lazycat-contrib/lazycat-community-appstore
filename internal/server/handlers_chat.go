package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/chatconversation"
	"lazycat.community/appstore/ent/chatmessage"
	"lazycat.community/appstore/ent/chatparticipant"
	"lazycat.community/appstore/ent/user"
)

type chatActor struct {
	User         *entgo.User
	UserID       int
	ClientUserID string
	DisplayName  string
	AvatarURL    string
	IsClient     bool
}

type chatStartRequest struct {
	TargetUserID int    `json:"targetUserId"`
	AppID        *int   `json:"appId,omitempty"`
	Body         string `json:"body,omitempty"`
}

type chatMessageRequest struct {
	Body string `json:"body"`
}

type chatConversationDTO struct {
	ID                    int                  `json:"id"`
	AppID                 *int                 `json:"appId,omitempty"`
	AppName               string               `json:"appName,omitempty"`
	Topic                 string               `json:"topic"`
	Origin                string               `json:"origin"`
	Participants          []chatParticipantDTO `json:"participants"`
	Peer                  *chatParticipantDTO  `json:"peer,omitempty"`
	LastMessageBody       string               `json:"lastMessageBody"`
	LastMessageSenderName string               `json:"lastMessageSenderName"`
	LastMessageAt         *time.Time           `json:"lastMessageAt,omitempty"`
	UnreadCount           int                  `json:"unreadCount"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

type chatParticipantDTO struct {
	ActorType    string `json:"actorType"`
	UserID       int    `json:"userId,omitempty"`
	ClientUserID string `json:"clientUserId,omitempty"`
	DisplayName  string `json:"displayName"`
	AvatarURL    string `json:"avatarUrl,omitempty"`
	IsSelf       bool   `json:"isSelf"`
}

type chatMessageDTO struct {
	ID                 int       `json:"id"`
	ConversationID     int       `json:"conversationId"`
	SenderType         string    `json:"senderType"`
	SenderUserID       int       `json:"senderUserId,omitempty"`
	SenderClientUserID string    `json:"senderClientUserId,omitempty"`
	SenderName         string    `json:"senderName"`
	SenderAvatarURL    string    `json:"senderAvatarUrl,omitempty"`
	Body               string    `json:"body"`
	IsMine             bool      `json:"isMine"`
	CreatedAt          time.Time `json:"createdAt"`
}

func (s *Server) handleListChatUsers(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	if !s.chatEnabled(r.Context()) {
		writeError(w, http.StatusForbidden, "CHAT_DISABLED", "Chat is disabled", nil)
		return
	}
	records, err := s.db.User.Query().
		Where(user.DisabledEQ(false), user.IDNEQ(u.ID)).
		Order(entgo.Asc(user.FieldUsername)).
		Limit(200).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_USER_LIST_FAILED", "Could not list chat users", nil)
		return
	}
	out := make([]publicUser, 0, len(records))
	for _, record := range records {
		out = append(out, toPublicUser(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) handleListChatConversations(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	s.cleanupExpiredChat(r.Context())
	participants, err := s.chatParticipantQueryForActor(actor).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_LIST_FAILED", "Could not list conversations", nil)
		return
	}
	conversationIDs := make([]int, 0, len(participants))
	selfByConversation := map[int]*entgo.ChatParticipant{}
	for _, participant := range participants {
		conversationIDs = append(conversationIDs, participant.ConversationID)
		selfByConversation[participant.ConversationID] = participant
	}
	if len(conversationIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"conversations": []chatConversationDTO{}})
		return
	}
	records, err := s.db.ChatConversation.Query().
		Where(chatconversation.IDIn(conversationIDs...)).
		Order(entgo.Desc(chatconversation.FieldLastMessageAt), entgo.Desc(chatconversation.FieldUpdatedAt)).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_LIST_FAILED", "Could not list conversations", nil)
		return
	}
	allParticipants, _ := s.db.ChatParticipant.Query().Where(chatparticipant.ConversationIDIn(conversationIDs...)).All(r.Context())
	participantsByConversation := map[int][]*entgo.ChatParticipant{}
	for _, participant := range allParticipants {
		participantsByConversation[participant.ConversationID] = append(participantsByConversation[participant.ConversationID], participant)
	}
	out := make([]chatConversationDTO, 0, len(records))
	for _, record := range records {
		self := selfByConversation[record.ID]
		if self == nil || chatConversationHiddenForParticipant(record, self) {
			continue
		}
		out = append(out, s.chatConversationDTO(r.Context(), record, actor, self, participantsByConversation[record.ID]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

func (s *Server) handleCreateChatConversation(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	if actor.IsClient {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Client chat must start from an app", nil)
		return
	}
	var input chatStartRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	record, created, ok := s.createOrLoadConversation(w, r, actor, input.TargetUserID, input.AppID)
	if !ok {
		return
	}
	if body := cleanChatMessageBody(input.Body); body != "" {
		if _, ok := s.createChatMessage(w, r, actor, record.ID, body); !ok {
			return
		}
		record, _ = s.db.ChatConversation.Get(r.Context(), record.ID)
	}
	dto, ok := s.chatConversationDTOForActor(r.Context(), record.ID, actor)
	if !ok {
		writeError(w, http.StatusInternalServerError, "CHAT_CREATE_FAILED", "Could not load conversation", nil)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"conversation": dto})
}

func (s *Server) handleCreateAppChatConversation(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || appID <= 0 {
		badRequest(w, err)
		return
	}
	appRecord, err := s.db.App.Get(r.Context(), appID)
	if err != nil || appRecord.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.chatActorCanSeeApp(r, actor, appRecord) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	var input chatStartRequest
	_ = decodeJSON(r, &input)
	record, created, ok := s.createOrLoadConversation(w, r, actor, appRecord.OwnerID, &appRecord.ID)
	if !ok {
		return
	}
	if body := cleanChatMessageBody(input.Body); body != "" {
		if _, ok := s.createChatMessage(w, r, actor, record.ID, body); !ok {
			return
		}
		record, _ = s.db.ChatConversation.Get(r.Context(), record.ID)
	}
	dto, ok := s.chatConversationDTOForActor(r.Context(), record.ID, actor)
	if !ok {
		writeError(w, http.StatusInternalServerError, "CHAT_CREATE_FAILED", "Could not load conversation", nil)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"conversation": dto})
}

func (s *Server) handleListChatMessages(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	conversationID, ok := chatConversationIDFromPath(w, r)
	if !ok {
		return
	}
	participant, ok := s.chatParticipantForActor(r.Context(), conversationID, actor)
	if !ok {
		writeError(w, http.StatusNotFound, "CHAT_NOT_FOUND", "Conversation not found", nil)
		return
	}
	query := s.db.ChatMessage.Query().
		Where(chatmessage.ConversationIDEQ(conversationID), chatmessage.DeletedEQ(false)).
		Order(entgo.Asc(chatmessage.FieldCreatedAt), entgo.Asc(chatmessage.FieldID))
	if participant.HiddenAt != nil {
		query.Where(chatmessage.CreatedAtGT(*participant.HiddenAt))
	}
	records, err := query.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_MESSAGE_LIST_FAILED", "Could not list messages", nil)
		return
	}
	out := make([]chatMessageDTO, 0, len(records))
	for _, record := range records {
		out = append(out, chatMessageDTO{
			ID:                 record.ID,
			ConversationID:     record.ConversationID,
			SenderType:         string(record.SenderType),
			SenderUserID:       record.SenderUserID,
			SenderClientUserID: record.SenderClientUserID,
			SenderName:         record.SenderName,
			SenderAvatarURL:    record.SenderAvatarURL,
			Body:               record.Body,
			IsMine:             actor.matchesMessage(record),
			CreatedAt:          record.CreatedAt,
		})
	}
	now := time.Now()
	_, _ = s.db.ChatParticipant.UpdateOneID(participant.ID).SetLastReadAt(now).Save(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"messages": out})
}

func (s *Server) handleCreateChatMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	conversationID, ok := chatConversationIDFromPath(w, r)
	if !ok {
		return
	}
	var input chatMessageRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	body := cleanChatMessageBody(input.Body)
	if body == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Message body is required", nil)
		return
	}
	message, ok := s.createChatMessage(w, r, actor, conversationID, body)
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": chatMessageDTO{
		ID:                 message.ID,
		ConversationID:     message.ConversationID,
		SenderType:         string(message.SenderType),
		SenderUserID:       message.SenderUserID,
		SenderClientUserID: message.SenderClientUserID,
		SenderName:         message.SenderName,
		SenderAvatarURL:    message.SenderAvatarURL,
		Body:               message.Body,
		IsMine:             true,
		CreatedAt:          message.CreatedAt,
	}})
}

func (s *Server) handleReadChatConversation(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	conversationID, ok := chatConversationIDFromPath(w, r)
	if !ok {
		return
	}
	participant, ok := s.chatParticipantForActor(r.Context(), conversationID, actor)
	if !ok {
		writeError(w, http.StatusNotFound, "CHAT_NOT_FOUND", "Conversation not found", nil)
		return
	}
	_, _ = s.db.ChatParticipant.UpdateOneID(participant.ID).SetLastReadAt(time.Now()).Save(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteChatConversation(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	conversationID, ok := chatConversationIDFromPath(w, r)
	if !ok {
		return
	}
	participant, ok := s.chatParticipantForActor(r.Context(), conversationID, actor)
	if !ok {
		writeError(w, http.StatusNotFound, "CHAT_NOT_FOUND", "Conversation not found", nil)
		return
	}
	now := time.Now()
	_, _ = s.db.ChatParticipant.UpdateOneID(participant.ID).SetHiddenAt(now).SetLastReadAt(now).Save(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatEvents(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.chatActorFromRequest(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "SSE_UNSUPPORTED", "Streaming is not supported", nil)
		return
	}
	events, unsubscribe := s.chatHub.subscribe()
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case event := <-events:
			if event.ConversationID > 0 {
				if _, visible := s.chatParticipantForActor(r.Context(), event.ConversationID, actor); !visible {
					continue
				}
			}
			raw, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: chat\n")
			fmt.Fprintf(w, "data: %s\n\n", raw)
			flusher.Flush()
		}
	}
}

func (s *Server) createOrLoadConversation(w http.ResponseWriter, r *http.Request, actor chatActor, targetUserID int, appID *int) (*entgo.ChatConversation, bool, bool) {
	if targetUserID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Target user is required", nil)
		return nil, false, false
	}
	if !actor.IsClient && actor.UserID == targetUserID {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "You cannot start a chat with yourself", nil)
		return nil, false, false
	}
	target, err := s.db.User.Get(r.Context(), targetUserID)
	if err != nil || target.Disabled {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return nil, false, false
	}
	var appRecord *entgo.App
	if appID != nil && *appID > 0 {
		appRecord, err = s.db.App.Get(r.Context(), *appID)
		if err != nil || appRecord.Status != app.StatusAPPROVED {
			writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
			return nil, false, false
		}
		if !s.chatActorCanSeeApp(r, actor, appRecord) {
			writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
			return nil, false, false
		}
	}
	if existing, ok := s.findConversationBetween(r.Context(), actor, targetUserID, appID); ok {
		update := s.db.ChatParticipant.Update().Where(chatparticipant.ConversationIDEQ(existing.ID))
		if actor.IsClient {
			update.Where(chatparticipant.ActorTypeEQ(chatparticipant.ActorTypeCLIENT), chatparticipant.ClientUserIDEQ(actor.ClientUserID))
		} else {
			update.Where(chatparticipant.ActorTypeEQ(chatparticipant.ActorTypeUSER), chatparticipant.UserIDEQ(actor.UserID))
		}
		_, _ = update.ClearHiddenAt().Save(r.Context())
		return existing, false, true
	}
	topic := ""
	if appRecord != nil {
		topic = appRecord.Name
	}
	tx, err := s.db.Tx(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_CREATE_FAILED", "Could not create conversation", nil)
		return nil, false, false
	}
	create := tx.ChatConversation.Create().SetTopic(topic)
	if appRecord != nil {
		create.SetAppID(appRecord.ID)
	}
	conversation, err := create.Save(r.Context())
	if err != nil {
		_ = tx.Rollback()
		writeError(w, http.StatusInternalServerError, "CHAT_CREATE_FAILED", "Could not create conversation", nil)
		return nil, false, false
	}
	if err := createChatParticipant(r.Context(), tx, conversation.ID, actor); err != nil {
		_ = tx.Rollback()
		writeError(w, http.StatusInternalServerError, "CHAT_CREATE_FAILED", "Could not create participant", nil)
		return nil, false, false
	}
	targetActor := chatActor{User: target, UserID: target.ID, DisplayName: userDisplayName(target), AvatarURL: target.AvatarURL}
	if err := createChatParticipant(r.Context(), tx, conversation.ID, targetActor); err != nil {
		_ = tx.Rollback()
		writeError(w, http.StatusInternalServerError, "CHAT_CREATE_FAILED", "Could not create participant", nil)
		return nil, false, false
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_CREATE_FAILED", "Could not create conversation", nil)
		return nil, false, false
	}
	s.chatHub.broadcast(chatEvent{Type: "conversation_updated", ConversationID: conversation.ID})
	return conversation, true, true
}

func createChatParticipant(ctx context.Context, tx *entgo.Tx, conversationID int, actor chatActor) error {
	create := tx.ChatParticipant.Create().
		SetConversationID(conversationID).
		SetDisplayName(actor.DisplayName).
		SetAvatarURL(actor.AvatarURL)
	if actor.IsClient {
		create.SetActorType(chatparticipant.ActorTypeCLIENT).SetClientUserID(actor.ClientUserID)
	} else {
		create.SetActorType(chatparticipant.ActorTypeUSER).SetUserID(actor.UserID)
	}
	_, err := create.Save(ctx)
	return err
}

func (s *Server) createChatMessage(w http.ResponseWriter, r *http.Request, actor chatActor, conversationID int, body string) (*entgo.ChatMessage, bool) {
	participant, ok := s.chatParticipantForActor(r.Context(), conversationID, actor)
	if !ok {
		writeError(w, http.StatusNotFound, "CHAT_NOT_FOUND", "Conversation not found", nil)
		return nil, false
	}
	create := s.db.ChatMessage.Create().
		SetConversationID(conversationID).
		SetSenderName(actor.DisplayName).
		SetSenderAvatarURL(actor.AvatarURL).
		SetBody(body)
	if actor.IsClient {
		create.SetSenderType(chatmessage.SenderTypeCLIENT).SetSenderClientUserID(actor.ClientUserID)
	} else {
		create.SetSenderType(chatmessage.SenderTypeUSER).SetSenderUserID(actor.UserID)
	}
	message, err := create.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_MESSAGE_CREATE_FAILED", "Could not create message", nil)
		return nil, false
	}
	now := message.CreatedAt
	_, _ = s.db.ChatConversation.UpdateOneID(conversationID).
		SetLastMessageBody(trimRunes(body, 180)).
		SetLastMessageSenderName(actor.DisplayName).
		SetLastMessageAt(now).
		Save(r.Context())
	_, _ = s.db.ChatParticipant.UpdateOneID(participant.ID).SetLastReadAt(now).ClearHiddenAt().Save(r.Context())
	_, _ = s.db.ChatParticipant.Update().
		Where(chatparticipant.ConversationIDEQ(conversationID)).
		ClearHiddenAt().
		Save(r.Context())
	s.chatHub.broadcast(chatEvent{Type: "conversation_updated", ConversationID: conversationID})
	return message, true
}

func (s *Server) chatActorFromRequest(w http.ResponseWriter, r *http.Request) (chatActor, bool) {
	if !s.chatEnabled(r.Context()) {
		writeError(w, http.StatusForbidden, "CHAT_DISABLED", "Chat is disabled", nil)
		return chatActor{}, false
	}
	if u, ok := s.authenticate(r); ok && !s.emailVerificationRequiredForUser(r.Context(), u) {
		return chatActor{User: u, UserID: u.ID, DisplayName: userDisplayName(u), AvatarURL: u.AvatarURL}, true
	}
	if !s.cfg.TrustLazyCatClientChat || r.Header.Get("X-LazyCat-Client-Proxy") != "lazycat-appstore-client" || sanitizeIdentity(r.Header.Get("X-LazyCat-Client-Device-ID")) == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return chatActor{}, false
	}
	clientUserID := sanitizeIdentity(r.Header.Get("X-LazyCat-Client-User-ID"))
	if clientUserID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return chatActor{}, false
	}
	if !s.sourcePasswordAllowsClientComment(r) {
		writeError(w, http.StatusUnauthorized, "SOURCE_PASSWORD_REQUIRED", "A valid source password is required", nil)
		return chatActor{}, false
	}
	displayName := sanitizeDisplayName(r.Header.Get("X-LazyCat-Client-Display-Name"))
	if displayName == "" {
		displayName = "LazyCat " + trimRunes(clientUserID, 12)
	}
	return chatActor{ClientUserID: clientUserID, DisplayName: displayName, IsClient: true}, true
}

func (s *Server) chatActorCanSeeApp(r *http.Request, actor chatActor, record *entgo.App) bool {
	if s.userCanSeeApp(r, record, actor.User) {
		return true
	}
	allowed, err := s.requestHasGroupCodeForApp(r, record.ID)
	return err == nil && allowed
}

func (s *Server) chatParticipantQueryForActor(actor chatActor) *entgo.ChatParticipantQuery {
	q := s.db.ChatParticipant.Query()
	if actor.IsClient {
		return q.Where(chatparticipant.ActorTypeEQ(chatparticipant.ActorTypeCLIENT), chatparticipant.ClientUserIDEQ(actor.ClientUserID))
	}
	return q.Where(chatparticipant.ActorTypeEQ(chatparticipant.ActorTypeUSER), chatparticipant.UserIDEQ(actor.UserID))
}

func (s *Server) chatParticipantForActor(ctx context.Context, conversationID int, actor chatActor) (*entgo.ChatParticipant, bool) {
	participant, err := s.chatParticipantQueryForActor(actor).
		Where(chatparticipant.ConversationIDEQ(conversationID)).
		Only(ctx)
	if err != nil {
		return nil, false
	}
	return participant, true
}

func (s *Server) findConversationBetween(ctx context.Context, actor chatActor, targetUserID int, appID *int) (*entgo.ChatConversation, bool) {
	participants, err := s.chatParticipantQueryForActor(actor).All(ctx)
	if err != nil || len(participants) == 0 {
		return nil, false
	}
	conversationIDs := make([]int, 0, len(participants))
	for _, participant := range participants {
		conversationIDs = append(conversationIDs, participant.ConversationID)
	}
	targetParticipants, err := s.db.ChatParticipant.Query().
		Where(chatparticipant.ConversationIDIn(conversationIDs...), chatparticipant.ActorTypeEQ(chatparticipant.ActorTypeUSER), chatparticipant.UserIDEQ(targetUserID)).
		All(ctx)
	if err != nil || len(targetParticipants) == 0 {
		return nil, false
	}
	targetConversationIDs := make([]int, 0, len(targetParticipants))
	for _, participant := range targetParticipants {
		targetConversationIDs = append(targetConversationIDs, participant.ConversationID)
	}
	query := s.db.ChatConversation.Query().Where(chatconversation.IDIn(targetConversationIDs...))
	if appID == nil || *appID <= 0 {
		query.Where(chatconversation.AppIDIsNil())
	} else {
		query.Where(chatconversation.AppIDEQ(*appID))
	}
	record, err := query.Order(entgo.Desc(chatconversation.FieldUpdatedAt)).First(ctx)
	return record, err == nil
}

func (s *Server) chatConversationDTOForActor(ctx context.Context, conversationID int, actor chatActor) (chatConversationDTO, bool) {
	participant, ok := s.chatParticipantForActor(ctx, conversationID, actor)
	if !ok {
		return chatConversationDTO{}, false
	}
	record, err := s.db.ChatConversation.Get(ctx, conversationID)
	if err != nil || chatConversationHiddenForParticipant(record, participant) {
		return chatConversationDTO{}, false
	}
	participants, _ := s.db.ChatParticipant.Query().Where(chatparticipant.ConversationIDEQ(conversationID)).All(ctx)
	return s.chatConversationDTO(ctx, record, actor, participant, participants), true
}

func (s *Server) chatConversationDTO(ctx context.Context, record *entgo.ChatConversation, actor chatActor, self *entgo.ChatParticipant, participants []*entgo.ChatParticipant) chatConversationDTO {
	dto := chatConversationDTO{
		ID:                    record.ID,
		AppID:                 record.AppID,
		Topic:                 record.Topic,
		Origin:                "site",
		LastMessageBody:       record.LastMessageBody,
		LastMessageSenderName: record.LastMessageSenderName,
		LastMessageAt:         record.LastMessageAt,
		CreatedAt:             record.CreatedAt,
		UpdatedAt:             record.UpdatedAt,
	}
	if record.AppID != nil {
		if appRecord, err := s.db.App.Get(ctx, *record.AppID); err == nil {
			dto.AppName = appRecord.Name
		}
	}
	for _, participant := range participants {
		item := chatParticipantDTO{
			ActorType:    string(participant.ActorType),
			UserID:       participant.UserID,
			ClientUserID: participant.ClientUserID,
			DisplayName:  participant.DisplayName,
			AvatarURL:    participant.AvatarURL,
			IsSelf:       actor.matchesParticipant(participant),
		}
		dto.Participants = append(dto.Participants, item)
		if !item.IsSelf && dto.Peer == nil {
			peer := item
			dto.Peer = &peer
		}
	}
	if dto.Peer == nil && len(dto.Participants) > 0 {
		peer := dto.Participants[0]
		dto.Peer = &peer
	}
	dto.UnreadCount = s.chatUnreadCount(ctx, record.ID, actor, self)
	return dto
}

func (s *Server) chatUnreadCount(ctx context.Context, conversationID int, actor chatActor, self *entgo.ChatParticipant) int {
	query := s.db.ChatMessage.Query().Where(chatmessage.ConversationIDEQ(conversationID), chatmessage.DeletedEQ(false))
	if actor.IsClient {
		query.Where(chatmessage.Or(chatmessage.SenderTypeNEQ(chatmessage.SenderTypeCLIENT), chatmessage.SenderClientUserIDNEQ(actor.ClientUserID)))
	} else {
		query.Where(chatmessage.Or(chatmessage.SenderTypeNEQ(chatmessage.SenderTypeUSER), chatmessage.SenderUserIDNEQ(actor.UserID)))
	}
	if self != nil && self.LastReadAt != nil {
		query.Where(chatmessage.CreatedAtGT(*self.LastReadAt))
	}
	if self != nil && self.HiddenAt != nil {
		query.Where(chatmessage.CreatedAtGT(*self.HiddenAt))
	}
	count, _ := query.Count(ctx)
	return count
}

func chatConversationHiddenForParticipant(record *entgo.ChatConversation, participant *entgo.ChatParticipant) bool {
	if participant == nil || participant.HiddenAt == nil {
		return false
	}
	if record.LastMessageAt == nil {
		return true
	}
	return !record.LastMessageAt.After(*participant.HiddenAt)
}

func (actor chatActor) matchesParticipant(participant *entgo.ChatParticipant) bool {
	if participant == nil {
		return false
	}
	if actor.IsClient {
		return participant.ActorType == chatparticipant.ActorTypeCLIENT && participant.ClientUserID == actor.ClientUserID
	}
	return participant.ActorType == chatparticipant.ActorTypeUSER && participant.UserID == actor.UserID
}

func (actor chatActor) matchesMessage(message *entgo.ChatMessage) bool {
	if message == nil {
		return false
	}
	if actor.IsClient {
		return message.SenderType == chatmessage.SenderTypeCLIENT && message.SenderClientUserID == actor.ClientUserID
	}
	return message.SenderType == chatmessage.SenderTypeUSER && message.SenderUserID == actor.UserID
}

func chatConversationIDFromPath(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid conversation id", nil)
		return 0, false
	}
	return id, true
}

func cleanChatMessageBody(value string) string {
	return trimRunes(strings.TrimSpace(value), 4000)
}

func (s *Server) cleanupExpiredChat(ctx context.Context) {
	days := s.chatRetentionDays(ctx)
	if days <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	_, _ = s.db.ChatMessage.Delete().Where(chatmessage.CreatedAtLT(cutoff)).Exec(ctx)
	_, _ = s.db.ChatConversation.Update().
		Where(chatconversation.LastMessageAtLT(cutoff)).
		SetLastMessageBody("").
		SetLastMessageSenderName("").
		ClearLastMessageAt().
		Save(ctx)
}
