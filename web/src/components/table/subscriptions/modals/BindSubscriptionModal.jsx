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

import React, { useMemo, useRef, useState } from 'react';
import {
  Button,
  Form,
  Select,
  SideSheet,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { IconClose, IconSave } from '@douyinfe/semi-icons';
import {
  API,
  semiSelectPortalProps,
  showError,
  showSuccess,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const BindSubscriptionModal = ({
  visible,
  handleClose,
  plans = [],
  refresh,
  t,
}) => {
  const [loading, setLoading] = useState(false);
  const formApiRef = useRef(null);
  const isMobile = useIsMobile();

  const planOptions = useMemo(() => {
    return (plans || [])
      .map((item) => item?.plan)
      .filter((plan) => plan?.id)
      .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0));
  }, [plans]);

  const handleSubmit = async (values) => {
    const userId = Number(values.user_id || 0);
    const planId = Number(values.plan_id || 0);
    if (!userId || !planId) {
      showError(t('参数错误'));
      return;
    }
    setLoading(true);
    try {
      const res = await API.post('/api/subscription/admin/bind', {
        user_id: userId,
        plan_id: planId,
      });
      if (res.data?.success) {
        const msg = res.data?.data?.message;
        showSuccess(msg || t('绑定成功'));
        handleClose();
        await refresh?.();
      } else {
        showError(res.data?.message || t('绑定失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <SideSheet
      title={
        <Space>
          <Title heading={5} className='!m-0'>
            {t('绑定订阅')}
          </Title>
          <Text type='tertiary' size='small'>
            {t('管理员发放')}
          </Text>
        </Space>
      }
      visible={visible}
      onCancel={handleClose}
      placement={isMobile ? 'bottom' : 'right'}
      width={isMobile ? '100%' : 480}
      height={isMobile ? '70%' : undefined}
      footer={
        <div className='flex justify-end gap-2'>
          <Button
            theme='light'
            icon={<IconClose />}
            onClick={handleClose}
            disabled={loading}
          >
            {t('取消')}
          </Button>
          <Button
            type='primary'
            icon={<IconSave />}
            loading={loading}
            onClick={() => formApiRef.current?.submitForm()}
          >
            {t('绑定')}
          </Button>
        </div>
      }
    >
      <Form
        labelPosition='top'
        getFormApi={(api) => (formApiRef.current = api)}
        onSubmit={handleSubmit}
      >
        <Form.InputNumber
          field='user_id'
          label={t('用户 ID')}
          min={1}
          precision={0}
          showClear
          rules={[{ required: true, message: t('请输入用户 ID') }]}
        />
        <Form.Select
          {...semiSelectPortalProps}
          field='plan_id'
          label={t('订阅套餐')}
          placeholder={t('请选择订阅套餐')}
          filter
          showClear
          rules={[{ required: true, message: t('请选择订阅套餐') }]}
        >
          {planOptions.map((plan) => (
            <Select.Option key={plan.id} value={plan.id}>
              {`#${plan.id} ${plan.title || t('未命名套餐')}`}
            </Select.Option>
          ))}
        </Form.Select>
      </Form>
    </SideSheet>
  );
};

export default BindSubscriptionModal;
