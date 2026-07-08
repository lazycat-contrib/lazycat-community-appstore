import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, clientApi } from '../../shared/api';
import type { ChatConversation, SourceApp, SourceSubscription, StoreApp, Toast, User } from '../../shared/types';
import { runAction, sourceForApp } from '../../shared/utils';
import type { ChatFocus } from './ChatWorkspace';

type UseChatEntryActionsOptions = {
  user: User | null;
  sources: SourceSubscription[];
  setToast: (toast: Toast) => void;
  onLoginRequired: () => void;
  onOpenChat: () => void;
  onCloseStoreDetail: () => void;
  onCloseSourceDetail: () => void;
};

export function useChatEntryActions({
  user,
  sources,
  setToast,
  onLoginRequired,
  onOpenChat,
  onCloseStoreDetail,
  onCloseSourceDetail,
}: UseChatEntryActionsOptions) {
  const { t } = useTranslation();
  const [chatFocus, setChatFocus] = useState<ChatFocus | null>(null);

  async function contactStorePublisher(app: StoreApp) {
    if (!user) {
      onLoginRequired();
      return;
    }
    await runAction(setToast, t('drawer.contactPublisherFailed'), async () => {
      const data = await api<{ conversation: ChatConversation }>(`/api/v1/apps/${app.id}/chat`, {
        method: 'POST',
        body: JSON.stringify({}),
      });
      setChatFocus({ id: data.conversation.id });
      onCloseStoreDetail();
      onOpenChat();
    });
  }

  async function contactSourcePublisher(app: SourceApp) {
    const source = sourceForApp(app, sources);
    if (!source || !source.chatAvailable || source.chatEnabled === false) {
      setToast({ tone: 'neutral', message: t('chat.sourceDisabled') });
      return;
    }
    await runAction(setToast, t('sourceDetail.contactPublisherFailed'), async () => {
      const data = await clientApi<{ conversation: ChatConversation }>(`/apps/${app.id}/chat`, {
        method: 'POST',
        body: JSON.stringify({}),
      });
      setChatFocus({ id: data.conversation.id, sourceId: source.id });
      onCloseSourceDetail();
      onOpenChat();
    });
  }

  return {
    chatFocus,
    consumeChatFocus: () => setChatFocus(null),
    contactStorePublisher,
    contactSourcePublisher,
  };
}
