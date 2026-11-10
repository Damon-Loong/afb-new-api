/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState } from 'react';
import { Modal } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';

const DeleteUserModal = ({
  visible,
  onCancel,
  onConfirm,
  user,
  users,
  activePage,
  refresh,
  manageUser,
  t,
}) => {
  const [saving, setSaving] = useState(false);
  const cancelled = user?.DeletedAt != null;
  const handleConfirm = async () => {
    if (!user || saving) return;
    if (cancelled) {
      setSaving(true);
      try {
        const { data } = await API.delete(`/api/user/${user.id}/purge`);
        if (!data.success) {
          showError(data.message);
          return;
        }
        showSuccess(t('删除成功'));
        onCancel();
        await refresh(
          users.length === 1 && activePage > 1 ? activePage - 1 : activePage,
        );
      } catch (error) {
        showError(
          error?.response?.data?.message || t('删除失败，请刷新列表后重试'),
        );
      } finally {
        setSaving(false);
      }
      return;
    }
    await manageUser(user.id, 'delete', user);
    await refresh();
    setTimeout(() => {
      if (users.length === 0 && activePage > 1) {
        refresh(activePage - 1);
      }
    }, 100);
    onCancel(); // Close modal after success
  };

  return (
    <Modal
      title={
        cancelled ? t('确定删除此用户？') : t('确定是否要注销此用户？')
      }
      visible={visible}
      onCancel={onCancel}
      onOk={handleConfirm}
      type='danger'
      confirmLoading={saving}
      okText={cancelled ? t('删除') : t('确定')}
      cancelButtonProps={{ disabled: saving }}
      closable={!saving}
      maskClosable={!saving}
    >
      {cancelled ? (
        <>
          <p>
            {t('用户')}：{user.username}（ID {user.id}）
          </p>
          <p>
            {t(
              '删除后该账号将从用户列表中永久移除，无法恢复。历史账单和使用日志继续保留。',
            )}
          </p>
        </>
      ) : (
        t('相当于删除用户，此修改将不可逆')
      )}
    </Modal>
  );
};

export default DeleteUserModal;
