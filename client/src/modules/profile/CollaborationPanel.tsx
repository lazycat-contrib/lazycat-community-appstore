import { useState } from 'react';
import { Check, ChevronRight, Copy, History, Link, LogOut, RefreshCw, Settings, Trash2, UserPlus, Users, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { AppIcon } from '../../components/AppIcon';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { CollaborationData, CollaboratorInvite, CollaboratorRequest, OwnedCollaboration, StoreApp, Toast, User } from '../../shared/types';
import { cx, formatDate, localizedAppName, runAction, statusKey } from '../../shared/utils';

type AppDetailMode = 'detail' | 'manage';

export function CollaborationPanel({
  data,
  currentUser,
  onOpen,
  onRefresh,
  onListRefresh,
  setToast,
}: {
  data: CollaborationData;
  currentUser: User;
  onOpen: (app: StoreApp, mode?: AppDetailMode) => void;
  onRefresh: () => Promise<void>;
  onListRefresh: (options?: { silent?: boolean }) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [collaboratorDrafts, setCollaboratorDrafts] = useState<Record<number, string>>({});
  const [inviteDrafts, setInviteDrafts] = useState<Record<number, { email: string; sendEmail: boolean }>>({});

  const refreshCollaboration = async () => {
    await onRefresh();
    await onListRefresh({ silent: true });
  };
  const ownedCollaboration = data.owned.filter(
    (item) => item.collaborators.length > 0 || item.requests.length > 0 || item.invites.length > 0,
  );

  async function copyInvite(invite: CollaboratorInvite) {
    await runAction(setToast, t('profile.inviteCopyFailed'), async () => {
      if (!invite.inviteUrl || !navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(invite.inviteUrl);
      setToast({ tone: 'success', message: t('profile.inviteCopied') });
    });
  }

  async function addCollaborator(item: OwnedCollaboration) {
    const identity = (collaboratorDrafts[item.app.id] || '').trim();
    if (!identity) return;
    const body = identity.includes('@') ? { email: identity } : { username: identity };
    await runAction(setToast, t('profile.addCollaboratorFailed'), async () => {
      await api(`/api/v1/apps/${item.app.id}/collaborators`, {
        method: 'POST',
        body: JSON.stringify(body),
      });
      setCollaboratorDrafts((current) => ({ ...current, [item.app.id]: '' }));
      setToast({ tone: 'success', message: t('profile.collaboratorAdded') });
      await refreshCollaboration();
    });
  }

  async function removeCollaborator(appID: number, userID: number, self = false) {
    await runAction(setToast, self ? t('profile.leaveCollaborationFailed') : t('profile.removeCollaboratorFailed'), async () => {
      await api(`/api/v1/apps/${appID}/collaborators/${userID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: self ? t('profile.collaborationLeft') : t('profile.collaboratorRemoved') });
      await refreshCollaboration();
    });
  }

  async function createInvite(item: OwnedCollaboration) {
    const draft = inviteDrafts[item.app.id] || { email: '', sendEmail: false };
    await runAction(setToast, t('profile.createInviteFailed'), async () => {
      const data = await api<{ invite: CollaboratorInvite; inviteUrl: string }>(`/api/v1/apps/${item.app.id}/collaborator-invites`, {
        method: 'POST',
        body: JSON.stringify({ email: draft.email.trim(), sendEmail: draft.sendEmail }),
      });
      setInviteDrafts((current) => ({ ...current, [item.app.id]: { email: '', sendEmail: false } }));
      setToast({ tone: 'success', message: t('profile.inviteCreated') });
      if (data.inviteUrl && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(data.inviteUrl);
        setToast({ tone: 'success', message: t('profile.inviteCopied') });
      }
      await refreshCollaboration();
    });
  }

  async function decideRequest(request: CollaboratorRequest, approve: boolean) {
    await runAction(setToast, t('profile.requestDecisionFailed'), async () => {
      await api(`/api/v1/collaborator-requests/${request.id}/${approve ? 'approve' : 'reject'}`, { method: 'POST' });
      setToast({ tone: approve ? 'success' : 'neutral', message: approve ? t('profile.requestApproved') : t('profile.requestRejected') });
      await refreshCollaboration();
    });
  }

  return (
    <section className="collaboration-workspace">
      <div className="page-heading compact-heading">
        <span className="eyebrow subtle">{t('profile.tabs.collaboration')}</span>
        <h2>{t('profile.collaborationTitle')}</h2>
        <p>{t('profile.collaborationBody')}</p>
      </div>

      <section className="panel">
        <div className="section-title with-action">
          <div>
            <Users size={19} />
            <h2>{t('profile.collaboratingApps')}</h2>
          </div>
          <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={17} />} onClick={() => void refreshCollaboration()} />
        </div>
        {data.collaborating.length === 0 ? (
          <EmptyState icon={Users} title={t('profile.noCollaboratingApps')} />
        ) : (
          <XList className="action-list" density="compact" hasDividers>
            {data.collaborating.map((item) => {
              const appName = localizedAppName(item);
              return (
                <XListItem
                  className="collaboration-row"
                  key={item.id}
                  label={appName}
                  description={`${item.owner} · ${item.latestVersion?.version || t('app.noPublishedVersion')}`}
                  endContent={(
                    <div className="row-actions">
                      <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.openSubmission')} tooltip={t('profile.openSubmission')} icon={<ChevronRight size={17} />} onClick={() => void onOpen(item)} />
                      <XIconButton className="fixed-row-icon-button" type="button" variant="secondary" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item, 'manage')} />
                      <XIconButton className="fixed-row-icon-button" type="button" variant="destructive" size="sm" label={t('profile.leaveCollaboration')} tooltip={t('profile.leaveCollaboration')} icon={<LogOut size={17} />} onClick={() => void removeCollaborator(item.id, currentUser.id, true)} />
                    </div>
                  )}
                />
              );
            })}
          </XList>
        )}
      </section>

      <section className="panel">
        <SectionTitle icon={UserPlus} title={t('profile.ownedCollaboration')} />
        <div className="collaboration-owned-list">
          {ownedCollaboration.length === 0 ? (
            <EmptyState icon={UserPlus} title={t('profile.noOwnedCollaboration')} />
          ) : (
            ownedCollaboration.map((item) => {
              const inviteDraft = inviteDrafts[item.app.id] || { email: '', sendEmail: false };
              const appName = localizedAppName(item.app);
              return (
                <article className="nested-panel collaboration-app-panel" key={item.app.id}>
                  <div className="section-title with-action">
                    <div>
                      <AppIcon src={item.app.iconUrl} seed={item.app.packageId || item.app.slug || item.app.name} title={appName} size={36} />
                      <div>
                        <h3>{appName}</h3>
                        <span>{item.app.latestVersion?.version || t('app.noPublishedVersion')}</span>
                      </div>
                    </div>
                    <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item.app, 'manage')} />
                  </div>

                  <div className="collaboration-inline-form">
                    <XTextInput
                      label={t('profile.collaboratorIdentity')}
                      isLabelHidden
                      placeholder={t('profile.collaboratorIdentity')}
                      value={collaboratorDrafts[item.app.id] || ''}
                      onChange={(value) => setCollaboratorDrafts((current) => ({ ...current, [item.app.id]: value }))}
                    />
                    <XButton type="button" variant="secondary" size="sm" label={t('profile.addCollaborator')} icon={<UserPlus size={17} />} onClick={() => void addCollaborator(item)} />
                  </div>

                  <section className="collaboration-block">
                    <h4>{t('profile.collaboratorMembers')}</h4>
                    {item.collaborators.length === 0 ? (
                      <span className="muted-text">{t('profile.noCollaborators')}</span>
                    ) : (
                      <XList className="action-list compact-review-list" density="compact" hasDividers>
                        {item.collaborators.map((collaborator) => (
                          <XListItem
                            className="compact-row"
                            key={collaborator.id}
                            label={collaborator.username || t('drawer.userLabel', { id: collaborator.userId })}
                            description={collaborator.email || formatDate(collaborator.createdAt)}
                            endContent={<XIconButton className="fixed-row-icon-button" type="button" variant="destructive" size="sm" label={t('profile.removeCollaborator')} tooltip={t('profile.removeCollaborator')} icon={<Trash2 size={17} />} onClick={() => void removeCollaborator(item.app.id, collaborator.userId)} />}
                          />
                        ))}
                      </XList>
                    )}
                  </section>

                  <section className="collaboration-block">
                    <h4>{t('profile.collaboratorInvites')}</h4>
                    <div className="collaboration-inline-form invite-form">
                      <XTextInput
                        type="email"
                        label={t('profile.inviteEmail')}
                        description={t('profile.inviteEmailHelp')}
                        placeholder={t('profile.inviteEmail')}
                        value={inviteDraft.email}
                        onChange={(value) => setInviteDrafts((current) => ({ ...current, [item.app.id]: { ...inviteDraft, email: value } }))}
                      />
                      <XSwitch
                        label={t('profile.sendInviteEmail')}
                        value={inviteDraft.sendEmail}
                        width="100%"
                        onChange={(checked) => setInviteDrafts((current) => ({ ...current, [item.app.id]: { ...inviteDraft, sendEmail: checked } }))}
                      />
                      <XButton type="button" variant="secondary" size="sm" label={t('profile.createInvite')} icon={<Link size={17} />} onClick={() => void createInvite(item)} />
                    </div>
                    {item.invites.length === 0 ? (
                      <span className="muted-text">{t('profile.noActiveInvites')}</span>
                    ) : (
                      <XList className="action-list compact-review-list" density="compact" hasDividers>
                        {item.invites.map((invite) => (
                          <XListItem
                            className="compact-row"
                            key={invite.id}
                            label={invite.email || invite.tokenPrefix}
                            description={t('profile.inviteExpires', { date: formatDate(invite.expiresAt) })}
                            endContent={<XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.copyInvite')} tooltip={t('profile.copyInvite')} icon={<Copy size={17} />} onClick={() => void copyInvite(invite)} />}
                          />
                        ))}
                      </XList>
                    )}
                  </section>

                  <section className="collaboration-block">
                    <h4>{t('profile.collaboratorRequests')}</h4>
                    {item.requests.length === 0 ? (
                      <span className="muted-text">{t('drawer.noCollaboratorRequests')}</span>
                    ) : (
                      <XList className="action-list compact-review-list" density="compact" hasDividers>
                        {item.requests.map((request) => (
                          <XListItem
                            className="compact-row"
                            key={request.id}
                            label={request.username || t('drawer.userLabel', { id: request.userId || request.user_id || '-' })}
                            description={request.message || request.email || t('drawer.noMessage')}
                            endContent={(
                              <div className="row-actions">
                                <XIconButton className="fixed-row-icon-button" type="button" variant="secondary" size="sm" label={t('drawer.approveCollaborator')} tooltip={t('drawer.approveCollaborator')} icon={<Check size={17} />} onClick={() => void decideRequest(request, true)} />
                                <XIconButton className="fixed-row-icon-button" type="button" variant="destructive" size="sm" label={t('drawer.rejectCollaborator')} tooltip={t('drawer.rejectCollaborator')} icon={<X size={17} />} onClick={() => void decideRequest(request, false)} />
                              </div>
                            )}
                          />
                        ))}
                      </XList>
                    )}
                  </section>
                </article>
              );
            })
          )}
        </div>
      </section>

      <section className="panel">
        <SectionTitle icon={History} title={t('profile.outgoingRequests')} />
        {data.outgoingRequests.length === 0 ? (
          <EmptyState icon={History} title={t('profile.noOutgoingRequests')} />
        ) : (
          <XList className="action-list" density="compact" hasDividers>
            {data.outgoingRequests.map((request) => (
              <XListItem
                key={request.id}
                label={request.appName || t('common.app')}
                description={request.message || t('drawer.noMessage')}
                endContent={<StatusBadge tone={statusKey(request.status)} label={t(`statusLabels.${statusKey(request.status)}`)} />}
              />
            ))}
          </XList>
        )}
      </section>
    </section>
  );
}
