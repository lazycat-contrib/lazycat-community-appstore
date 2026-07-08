import { type FormEvent, useState } from 'react';
import { Users, UserRound } from 'lucide-react';
import { Tab as XTab, TabList as XTabList } from '@astryxdesign/core/TabList';
import { useTranslation } from 'react-i18next';
import type { Pagination, Toast, User } from '../../shared/types';
import { AdminGroupsPanel } from './AdminGroupsPanel';
import { AdminUsersPanel, type ManagedUserDraft } from './AdminUsersPanel';

export function AdminUsersWorkspace({
  users,
  userPagination,
  pageSizeOptions,
  userDraft,
  userDialogMode,
  userRoleOptions,
  sourceURL,
  setUserDraft,
  setUserDialogMode,
  openCreateUserDialog,
  openEditUserDialog,
  saveManagedUser,
  toggleUserDisabled,
  deleteManagedUser,
  loadUsersPage,
  setToast,
}: {
  users: User[];
  userPagination: Pagination;
  pageSizeOptions: number[];
  userDraft: ManagedUserDraft;
  userDialogMode: 'create' | 'edit' | null;
  userRoleOptions: Array<{ value: User['role']; label: string }>;
  sourceURL: string;
  setUserDraft: (draft: ManagedUserDraft) => void;
  setUserDialogMode: (mode: 'create' | 'edit' | null) => void;
  openCreateUserDialog: () => void;
  openEditUserDialog: (item: User) => void;
  saveManagedUser: (event: FormEvent) => Promise<void>;
  toggleUserDisabled: (item: User) => Promise<void>;
  deleteManagedUser: (item: User) => Promise<void>;
  loadUsersPage: (page: number, pageSize?: number) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<'users' | 'groups'>('users');

  return (
    <section className="workspace-pane admin-users-workspace">
      <div className="horizontal-control-scroll">
        <XTabList value={tab} onChange={(value) => setTab(value as 'users' | 'groups')} hasDivider size="sm">
          <XTab value="users" label={t('admin.usersTabs.users')} icon={<UserRound size={16} />} />
          <XTab value="groups" label={t('admin.usersTabs.groups')} icon={<Users size={16} />} />
        </XTabList>
      </div>
      {tab === 'users' ? (
        <AdminUsersPanel
          users={users}
          userPagination={userPagination}
          pageSizeOptions={pageSizeOptions}
          userDraft={userDraft}
          userDialogMode={userDialogMode}
          userRoleOptions={userRoleOptions}
          setUserDraft={setUserDraft}
          setUserDialogMode={setUserDialogMode}
          openCreateUserDialog={openCreateUserDialog}
          openEditUserDialog={openEditUserDialog}
          saveManagedUser={saveManagedUser}
          toggleUserDisabled={toggleUserDisabled}
          deleteManagedUser={deleteManagedUser}
          loadUsersPage={loadUsersPage}
        />
      ) : (
        <AdminGroupsPanel sourceURL={sourceURL} setToast={setToast} />
      )}
    </section>
  );
}
