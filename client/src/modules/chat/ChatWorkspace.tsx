import { useCallback, useEffect, useMemo, useState } from 'react';
import { MessageSquare, RefreshCw, Send, Trash2, UserPlus } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ChatComposer, ChatMessage, ChatMessageBubble, ChatMessageList, ChatMessageMetadata } from '@astryxdesign/core/Chat';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { useTranslation } from 'react-i18next';
import { API_BASE } from '../../config';
import { api, CLIENT_API_BASE, clientApi } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { ChatConversation, ChatMessage as ChatMessageRecord, SourceID, SourceSubscription, Toast, User } from '../../shared/types';
import { arrayOrEmpty, cx, errorMessage, formatDate } from '../../shared/utils';

type ChatMode = 'server' | 'client';

export type ChatFocus = {
  id: number;
  sourceId?: SourceID;
};

type ChatWorkspaceProps = {
  mode: ChatMode;
  sources?: SourceSubscription[];
  focus?: ChatFocus | null;
  onFocusConsumed?: () => void;
  setToast: (toast: Toast) => void;
};

type ConversationWithSource = ChatConversation & {
  sourceId?: SourceID;
  sourceName?: string;
};

function conversationKey(conversation: Pick<ConversationWithSource, 'id' | 'sourceId'>) {
  return `${conversation.sourceId ?? 'server'}:${conversation.id}`;
}

function isSourceChatEnabled(source: SourceSubscription) {
  return Boolean(source.chatAvailable && source.chatEnabled !== false);
}

export function ChatWorkspace({ mode, sources = [], focus, onFocusConsumed, setToast }: ChatWorkspaceProps) {
  const { t } = useTranslation();
  const enabledSources = useMemo(() => sources.filter(isSourceChatEnabled), [sources]);
  const [conversations, setConversations] = useState<ConversationWithSource[]>([]);
  const [activeKey, setActiveKey] = useState('');
  const [messages, setMessages] = useState<ChatMessageRecord[]>([]);
  const [draft, setDraft] = useState('');
  const [users, setUsers] = useState<User[]>([]);
  const [selectedUserID, setSelectedUserID] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isSending, setIsSending] = useState(false);
  const [isStarting, setIsStarting] = useState(false);

  const activeConversation = conversations.find((conversation) => conversationKey(conversation) === activeKey) || null;
  const activeConversationVersion = activeConversation
    ? `${conversationKey(activeConversation)}:${activeConversation.lastMessageAt || ''}:${activeConversation.updatedAt || ''}:${activeConversation.unreadCount || 0}`
    : '';
  const canUseChat = mode === 'server' || enabledSources.length > 0;
  const pageTitle = mode === 'server' ? t('chat.serverTitle') : t('chat.clientTitle');
  const pageBody = mode === 'server' ? t('chat.serverBody') : t('chat.clientBody');

  const loadConversations = useCallback(async () => {
    if (!canUseChat) {
      setConversations([]);
      setActiveKey('');
      return;
    }
    setIsLoading(true);
    try {
      if (mode === 'server') {
        const data = await api<{ conversations: ChatConversation[] }>('/api/v1/chat/conversations');
        const next = arrayOrEmpty(data.conversations);
        setConversations(next);
        setActiveKey((current) => current || (next[0] ? conversationKey(next[0]) : ''));
        return;
      }

      const results = await Promise.allSettled(
        enabledSources.map(async (source) => {
          const data = await clientApi<{ conversations: ChatConversation[] }>(
            `/chat/conversations?sourceId=${encodeURIComponent(String(source.id))}`,
          );
          return arrayOrEmpty(data.conversations).map((conversation) => ({
            ...conversation,
            sourceId: source.id,
            sourceName: source.name,
          }));
        }),
      );
      const next = results
        .flatMap((result) => (result.status === 'fulfilled' ? result.value : []))
        .sort((a, b) => Date.parse(b.lastMessageAt || b.updatedAt) - Date.parse(a.lastMessageAt || a.updatedAt));
      setConversations(next);
      setActiveKey((current) => current || (next[0] ? conversationKey(next[0]) : ''));
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('chat.loadFailed')) });
    } finally {
      setIsLoading(false);
    }
  }, [canUseChat, enabledSources, mode, setToast, t]);

  const loadMessages = useCallback(async (conversation: ConversationWithSource | null) => {
    if (!conversation) {
      setMessages([]);
      return;
    }
    try {
      if (mode === 'server') {
        const data = await api<{ messages: ChatMessageRecord[] }>(`/api/v1/chat/conversations/${conversation.id}/messages`);
        setMessages(arrayOrEmpty(data.messages));
        return;
      }
      const sourceID = conversation.sourceId;
      const data = await clientApi<{ messages: ChatMessageRecord[] }>(
        `/chat/conversations/${conversation.id}/messages?sourceId=${encodeURIComponent(String(sourceID))}`,
      );
      setMessages(arrayOrEmpty(data.messages));
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('chat.messageLoadFailed')) });
    }
  }, [mode, setToast, t]);

  useEffect(() => {
    void loadConversations();
  }, [loadConversations]);

  useEffect(() => {
    if (!focus) return;
    setActiveKey(`${focus.sourceId ?? 'server'}:${focus.id}`);
    onFocusConsumed?.();
  }, [focus, onFocusConsumed]);

  useEffect(() => {
    void loadMessages(activeConversation);
  }, [activeConversationVersion, loadMessages]);

  useEffect(() => {
    if (mode !== 'server') return;
    void api<{ users: User[] }>('/api/v1/chat/users')
      .then((data) => setUsers(arrayOrEmpty(data.users)))
      .catch(() => setUsers([]));
  }, [mode]);

  useEffect(() => {
    if (!canUseChat) return;
    const refreshChat = () => {
      void loadConversations();
      void loadMessages(activeConversation);
    };
    if (mode === 'server') {
      const events = new EventSource(`${API_BASE}/api/v1/chat/events`, { withCredentials: true });
      events.addEventListener('chat', refreshChat);
      events.onerror = () => undefined;
      return () => events.close();
    }
    const streams = enabledSources.map((source) => {
      const events = new EventSource(`${CLIENT_API_BASE}/chat/events?sourceId=${encodeURIComponent(String(source.id))}`, { withCredentials: true });
      events.addEventListener('chat', refreshChat);
      events.onerror = () => undefined;
      return events;
    });
    return () => streams.forEach((stream) => stream.close());
  }, [activeConversationVersion, activeConversation, canUseChat, enabledSources, loadConversations, loadMessages, mode]);

  const refreshActiveChat = useCallback(async () => {
    await loadConversations();
    await loadMessages(activeConversation);
  }, [activeConversation, loadConversations, loadMessages]);

  async function startUserConversation() {
    const targetUserId = Number(selectedUserID);
    if (!targetUserId) return;
    setIsStarting(true);
    try {
      const data = await api<{ conversation: ChatConversation }>('/api/v1/chat/conversations', {
        method: 'POST',
        body: JSON.stringify({ targetUserId }),
      });
      setSelectedUserID('');
      await loadConversations();
      setActiveKey(conversationKey(data.conversation));
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('chat.startFailed')) });
    } finally {
      setIsStarting(false);
    }
  }

  async function sendMessage(body: string) {
    const conversation = activeConversation;
    if (!conversation || !body.trim()) return;
    setIsSending(true);
    try {
      if (mode === 'server') {
        await api(`/api/v1/chat/conversations/${conversation.id}/messages`, {
          method: 'POST',
          body: JSON.stringify({ body }),
        });
      } else {
        await clientApi(`/chat/conversations/${conversation.id}/messages?sourceId=${encodeURIComponent(String(conversation.sourceId))}`, {
          method: 'POST',
          body: JSON.stringify({ body }),
        });
      }
      setDraft('');
      await Promise.all([loadConversations(), loadMessages(conversation)]);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('chat.sendFailed')) });
    } finally {
      setIsSending(false);
    }
  }

  async function deleteConversation() {
    const conversation = activeConversation;
    if (!conversation) return;
    try {
      if (mode === 'server') {
        await api(`/api/v1/chat/conversations/${conversation.id}`, { method: 'DELETE' });
      } else {
        await clientApi(`/chat/conversations/${conversation.id}?sourceId=${encodeURIComponent(String(conversation.sourceId))}`, { method: 'DELETE' });
      }
      setMessages([]);
      setActiveKey('');
      await loadConversations();
      setToast({ tone: 'neutral', message: t('chat.conversationCleared') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('chat.clearFailed')) });
    }
  }

  if (!canUseChat) {
    return (
      <section className="page-grid chat-page">
        <div className="page-heading">
          <span className="eyebrow subtle">{t('chat.eyebrow')}</span>
          <h1>{pageTitle}</h1>
          <p>{pageBody}</p>
        </div>
        <EmptyState icon={MessageSquare} title={t('chat.noClientSourcesTitle')} body={t('chat.noClientSourcesBody')} />
      </section>
    );
  }

  return (
    <section className="page-grid chat-page">
      <div className="page-heading with-action">
        <div>
          <span className="eyebrow subtle">{t('chat.eyebrow')}</span>
          <h1>{pageTitle}</h1>
          <p>{pageBody}</p>
        </div>
        <XIconButton type="button" variant="ghost" label={t('common.refresh')} icon={<RefreshCw size={18} />} onClick={() => void refreshActiveChat()} />
      </div>

      {mode === 'server' && (
        <section className="panel chat-start-panel">
          <SectionTitle icon={UserPlus} title={t('chat.startUserChat')} />
          <div className="chat-start-controls">
            <XSelector
              label={t('chat.user')}
              value={selectedUserID}
              options={[
                { value: '', label: t('chat.selectUser') },
                ...users.map((item) => ({ value: String(item.id), label: item.nickname || item.username })),
              ]}
              onChange={setSelectedUserID}
            />
            <XButton
              type="button"
              variant="primary"
              label={t('chat.start')}
              icon={<Send size={17} />}
              isDisabled={!selectedUserID || isStarting}
              onClick={() => void startUserConversation()}
            />
          </div>
        </section>
      )}

      <section className="chat-shell">
        <aside className="chat-conversation-list" aria-label={t('chat.conversationList')}>
          {isLoading ? (
            <div className="chat-list-loading">{t('common.loading')}</div>
          ) : conversations.length === 0 ? (
            <EmptyState icon={MessageSquare} title={t('chat.noConversations')} body={t('chat.noConversationsBody')} />
          ) : (
            conversations.map((conversation) => {
              const key = conversationKey(conversation);
              const peerName = conversation.peer?.displayName || conversation.topic || t('chat.unknownPeer');
              const origin = conversation.sourceName || (mode === 'server' ? t('chat.siteOrigin') : conversation.origin || t('common.source'));
              return (
                <button
                  type="button"
                  key={key}
                  className={cx('chat-conversation-item', activeKey === key && 'selected')}
                  onClick={() => setActiveKey(key)}
                >
                  <span className="chat-conversation-title">
                    <strong>{peerName}</strong>
                    {(conversation.unreadCount || 0) > 0 && <XBadge variant="error" label={conversation.unreadCount} />}
                  </span>
                  <span className="chat-conversation-origin">{origin}</span>
                  {conversation.appName && <span className="chat-conversation-app">{conversation.appName}</span>}
                  <span className="chat-conversation-preview">{conversation.lastMessageBody || t('chat.noMessagesYet')}</span>
                </button>
              );
            })
          )}
        </aside>

        <article className="chat-thread" aria-label={activeConversation?.peer?.displayName || t('chat.thread')}>
          {activeConversation ? (
            <>
              <header className="chat-thread-head">
                <div>
                  <strong>{activeConversation.peer?.displayName || activeConversation.topic || t('chat.unknownPeer')}</strong>
                  <span>{activeConversation.sourceName || t('chat.siteOrigin')}{activeConversation.appName ? ` · ${activeConversation.appName}` : ''}</span>
                </div>
                <div className="row-actions">
                  {(activeConversation.unreadCount || 0) > 0 && <StatusBadge tone="syncing" label={t('chat.unreadCount', { count: activeConversation.unreadCount })} />}
                  <XIconButton type="button" variant="destructive" label={t('chat.clearConversation')} icon={<Trash2 size={17} />} onClick={() => void deleteConversation()} />
                </div>
              </header>
              <div className="chat-message-area">
                <ChatMessageList
                  density="balanced"
                  emptyState={<EmptyState icon={MessageSquare} title={t('chat.noMessagesYet')} body={t('chat.noMessagesYetBody')} />}
                >
                  {messages.map((message) => (
                    <ChatMessage
                      key={message.id}
                      sender={message.isMine ? 'user' : 'assistant'}
                      name={message.senderName}
                    >
                      <ChatMessageBubble
                        metadata={(
                          <ChatMessageMetadata
                            timestamp={formatDate(message.createdAt)}
                            footer={message.isMine ? t('chat.me') : message.senderName}
                            status={message.isMine ? 'sent' : undefined}
                          />
                        )}
                      >
                        {message.body}
                      </ChatMessageBubble>
                    </ChatMessage>
                  ))}
                </ChatMessageList>
              </div>
              <ChatComposer
                value={draft}
                onChange={setDraft}
                onSubmit={(value) => void sendMessage(value)}
                placeholder={t('chat.composerPlaceholder')}
                isDisabled={isSending}
                density="compact"
              />
            </>
          ) : (
            <EmptyState icon={MessageSquare} title={t('chat.selectConversation')} body={t('chat.selectConversationBody')} />
          )}
        </article>
      </section>
    </section>
  );
}
