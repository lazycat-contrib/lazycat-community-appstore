import { type FormEvent } from 'react';
import { Pencil, Save, Trash2, UserPlus, UserRound, Users, X } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { UserAvatar } from '../../components/AppIcon';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import type { Pagination, User } from '../../shared/types';

export type ManagedUserDraft = {
  id?: number;
  username: string;
  nickname: string;
  email: string;
  password: string;
  role: User['role'];
  emailVerified: boolean;
  disabled: boolean;
};

export function emptyUserDraft(): ManagedUserDraft {
  return {
    username: '',
    nickname: '',
    email: '',
    password: '',
    role: 'USER',
    emailVerified: true,
    disabled: false,
  };
}

export function draftFromUser(user: User): ManagedUserDraft {
  return {
    id: user.id,
    username: user.username,
    nickname: user.nickname || '',
    email: user.email || '',
    password: '',
    role: user.role,
    emailVerified: user.emailVerified !== false,
    disabled: Boolean(user.disabled),
  };
}

function displayUserName(user: User | null | undefined) {
  return user?.nickname?.trim() || user?.username || '';
}

export function AdminUsersPanel({
  users,
  userPagination,
  pageSizeOptions,
  userDraft,
  userDialogMode,
  userRoleOptions,
  setUserDraft,
  setUserDialogMode,
  openCreateUserDialog,
  openEditUserDialog,
  saveManagedUser,
  toggleUserDisabled,
  deleteManagedUser,
  loadUsersPage,
}: {
  users: User[];
  userPagination: Pagination;
  pageSizeOptions: number[];
  userDraft: ManagedUserDraft;
  userDialogMode: 'create' | 'edit' | null;
  userRoleOptions: Array<{ value: User['role']; label: string }>;
  setUserDraft: (draft: ManagedUserDraft) => void;
  setUserDialogMode: (mode: 'create' | 'edit' | null) => void;
  openCreateUserDialog: () => void;
  openEditUserDialog: (item: User) => void;
  saveManagedUser: (event: FormEvent) => Promise<void>;
  toggleUserDisabled: (item: User) => Promise<void>;
  deleteManagedUser: (item: User) => Promise<void>;
  loadUsersPage: (page: number, pageSize?: number) => Promise<void>;
}) {
  const { t } = useTranslation();

  return (
    <section className="workspace-pane">
      <section className="panel">
        <div className="section-title with-action">
          <div>
            <Users size={19} />
            <h2>{t('admin.userManagement')}</h2>
          </div>
          <XButton type="button" variant="primary" size="sm" label={t('admin.createUser')} icon={<UserPlus size={17} />} onClick={openCreateUserDialog} />
        </div>
        {users.length === 0 ? (
          <EmptyState icon={Users} title={t('admin.noUsers')} />
        ) : (
          <XList className="action-list user-management-list" density="compact" hasDividers>
            {users.map((item) => (
              <XListItem
                className="user-row"
                key={item.id}
                startContent={<UserAvatar user={item} size={42} />}
                label={displayUserName(item)}
                description={`${item.username} · ${item.email || t('admin.noEmail')}`}
                endContent={(
                  <div className="row-actions">
                    <XBadge label={t(`admin.roles.${item.role === 'SITE_ADMIN' ? 'siteAdmin' : item.role === 'SOFTWARE_ADMIN' ? 'softwareAdmin' : 'user'}`)} variant={item.role === 'SITE_ADMIN' ? 'info' : item.role === 'SOFTWARE_ADMIN' ? 'success' : 'neutral'} />
                    {item.disabled && <XBadge label={t('admin.userDisabledBadge')} variant="error" />}
                    <XIconButton type="button" variant="ghost" size="sm" label={t('admin.editUserNamed', { name: displayUserName(item) })} icon={<Pencil size={16} />} onClick={() => openEditUserDialog(item)} />
                    <XIconButton type="button" variant="ghost" size="sm" label={item.disabled ? t('admin.enableUserNamed', { name: displayUserName(item) }) : t('admin.disableUserNamed', { name: displayUserName(item) })} icon={<UserRound size={16} />} onClick={() => void toggleUserDisabled(item)} />
                    <XIconButton type="button" variant="destructive" size="sm" label={t('admin.deleteUserNamed', { name: displayUserName(item) })} icon={<Trash2 size={16} />} onClick={() => void deleteManagedUser(item)} />
                  </div>
                )}
              />
            ))}
          </XList>
        )}
        {userPagination.pageSize > 0 && userPagination.totalItems > userPagination.pageSize && (
          <XPagination
            className="list-pagination"
            page={userPagination.page}
            onChange={(page) => void loadUsersPage(page, userPagination.pageSize)}
            totalItems={userPagination.totalItems}
            pageSize={userPagination.pageSize}
            pageSizeOptions={pageSizeOptions}
            onPageSizeChange={(pageSize) => void loadUsersPage(1, pageSize)}
            variant="pages"
            size="sm"
            label={t('pagination.label')}
          />
        )}
      </section>
      {userDialogMode && (
        <ModalLayer onClose={() => setUserDialogMode(null)} purpose="form">
          <form
            className="modal-panel form-panel user-dialog"
            aria-label={userDialogMode === 'create' ? t('admin.createUser') : t('admin.editUser')}
            onSubmit={(event) => void saveManagedUser(event)}
          >
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setUserDialogMode(null)} />
            <SectionTitle icon={userDialogMode === 'create' ? UserPlus : UserRound} title={userDialogMode === 'create' ? t('admin.createUser') : t('admin.editUser')} />
            <XFormLayout>
              <XTextInput label={t('common.username')} value={userDraft.username} onChange={(value) => setUserDraft({ ...userDraft, username: value })} />
              <XTextInput label={t('profile.nickname')} value={userDraft.nickname} onChange={(value) => setUserDraft({ ...userDraft, nickname: value })} />
              <XTextInput type="email" label={t('common.email')} value={userDraft.email} onChange={(value) => setUserDraft({ ...userDraft, email: value })} />
              <XTextInput type="password" label={userDialogMode === 'create' ? t('common.password') : t('admin.newPasswordOptional')} value={userDraft.password} onChange={(value) => setUserDraft({ ...userDraft, password: value })} />
              <XSelector label={t('common.role')} value={userDraft.role} options={userRoleOptions} onChange={(value) => setUserDraft({ ...userDraft, role: value as User['role'] })} />
              <XSelector
                label={t('admin.emailVerified')}
                value={String(userDraft.emailVerified)}
                options={[
                  { value: 'true', label: t('common.on') },
                  { value: 'false', label: t('common.off') },
                ]}
                onChange={(value) => setUserDraft({ ...userDraft, emailVerified: value === 'true' })}
              />
              <XSelector
                label={t('admin.userDisabledField')}
                value={String(userDraft.disabled)}
                options={[
                  { value: 'false', label: t('common.off') },
                  { value: 'true', label: t('common.on') },
                ]}
                onChange={(value) => setUserDraft({ ...userDraft, disabled: value === 'true' })}
              />
            </XFormLayout>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setUserDialogMode(null)} />
              <XButton type="submit" variant="primary" label={userDialogMode === 'create' ? t('admin.createUser') : t('common.save')} icon={<Save size={17} />} />
            </div>
          </form>
        </ModalLayer>
      )}
    </section>
  );
}
