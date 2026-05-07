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

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { Info } from 'lucide-react';

const toBoolean = (value) => value === true || value === 'true';

export default function SettingsPaymentGatewayWeChatPay(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle ? undefined : t('微信支付设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    WeChatPayEnabled: false,
    WeChatPayAppID: '',
    WeChatPayMchID: '',
    WeChatPayMchCertSerialNo: '',
    WeChatPayAPIv3Key: '',
    WeChatPayPrivateKey: '',
    WeChatPayNotifyURL: '',
    WeChatPayReturnURL: '',
    WeChatPayUnitPrice: 7.3,
    WeChatPayMinTopUp: 1,
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        WeChatPayEnabled: toBoolean(props.options.WeChatPayEnabled),
        WeChatPayAppID: props.options.WeChatPayAppID || '',
        WeChatPayMchID: props.options.WeChatPayMchID || '',
        WeChatPayMchCertSerialNo: props.options.WeChatPayMchCertSerialNo || '',
        WeChatPayAPIv3Key: props.options.WeChatPayAPIv3Key || '',
        WeChatPayPrivateKey: props.options.WeChatPayPrivateKey || '',
        WeChatPayNotifyURL: props.options.WeChatPayNotifyURL || '',
        WeChatPayReturnURL: props.options.WeChatPayReturnURL || '',
        WeChatPayUnitPrice:
          props.options.WeChatPayUnitPrice !== undefined
            ? parseFloat(props.options.WeChatPayUnitPrice)
            : 7.3,
        WeChatPayMinTopUp:
          props.options.WeChatPayMinTopUp !== undefined
            ? parseFloat(props.options.WeChatPayMinTopUp)
            : 1,
      };
      setInputs(currentInputs);
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitWeChatPay = async () => {
    if (props.options?.ServerAddress === '') {
      showError(t('请先填写服务器地址'));
      return;
    }

    setLoading(true);
    try {
      const options = [];

      options.push({
        key: 'WeChatPayEnabled',
        value: inputs.WeChatPayEnabled ? 'true' : 'false',
      });

      options.push({
        key: 'WeChatPayAppID',
        value: inputs.WeChatPayAppID || '',
      });
      options.push({
        key: 'WeChatPayMchID',
        value: inputs.WeChatPayMchID || '',
      });
      options.push({
        key: 'WeChatPayMchCertSerialNo',
        value: inputs.WeChatPayMchCertSerialNo || '',
      });

      if (inputs.WeChatPayAPIv3Key && inputs.WeChatPayAPIv3Key !== '') {
        options.push({
          key: 'WeChatPayAPIv3Key',
          value: inputs.WeChatPayAPIv3Key,
        });
      }
      if (inputs.WeChatPayPrivateKey && inputs.WeChatPayPrivateKey !== '') {
        options.push({
          key: 'WeChatPayPrivateKey',
          value: inputs.WeChatPayPrivateKey,
        });
      }

      options.push({
        key: 'WeChatPayNotifyURL',
        value: removeTrailingSlash(inputs.WeChatPayNotifyURL || ''),
      });
      options.push({
        key: 'WeChatPayReturnURL',
        value: removeTrailingSlash(inputs.WeChatPayReturnURL || ''),
      });
      options.push({
        key: 'WeChatPayUnitPrice',
        value: String(inputs.WeChatPayUnitPrice || 7.3),
      });
      options.push({
        key: 'WeChatPayMinTopUp',
        value: String(inputs.WeChatPayMinTopUp || 1),
      });

      const results = await Promise.all(
        options.map((opt) =>
          API.put('/api/option/', {
            key: opt.key,
            value: opt.value,
          }),
        ),
      );

      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length > 0) {
        errorResults.forEach((res) => showError(res.data.message));
      } else {
        showSuccess(t('更新成功'));
        props.refresh?.();
      }
    } catch (e) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Banner
            type='info'
            icon={<Info size={16} />}
            description={t(
              '微信支付使用 API v3（Native 扫码支付）。请确保回调地址为 HTTPS（生产环境要求）。',
            )}
            style={{ marginBottom: 16 }}
          />
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Switch
                field='WeChatPayEnabled'
                label={t('启用微信支付')}
                checkedText='｜'
                uncheckedText='〇'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayAppID'
                label={t('AppID')}
                placeholder={t('例如：wx1234567890abcdef')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayMchID'
                label={t('商户号 MchID')}
                placeholder={t('例如：190000****')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayMchCertSerialNo'
                label={t('商户证书序列号')}
                placeholder={t('例如：3775B6A4...')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayAPIv3Key'
                label={t('API v3 Key')}
                type='password'
                placeholder={t('敏感信息不会发送到前端显示')}
                extraText={t('填写后会覆盖服务端保存的密钥，留空表示保持不变')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.TextArea
                field='WeChatPayPrivateKey'
                label={t('商户私钥（PEM）')}
                type='password'
                placeholder={t('粘贴 apiclient_key.pem 内容，敏感信息不会发送到前端显示')}
                extraText={t('填写后会覆盖服务端保存的私钥，留空表示保持不变')}
                autosize={{ minRows: 3, maxRows: 6 }}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='WeChatPayUnitPrice'
                precision={2}
                label={t('充值价格（x元/美金）')}
                placeholder={t('例如：7.3')}
                min={0}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='WeChatPayMinTopUp'
                precision={4}
                step={0.01}
                label={t('最低充值数量')}
                placeholder={t('例如：1 或 0.01（测试）')}
                min={0.01}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayNotifyURL'
                label={t('回调地址（可选）')}
                placeholder={t('留空则使用默认 /api/wechatpay/webhook')}
              />
            </Col>
          </Row>

          <Row style={{ marginTop: 16 }}>
            <Col span={24}>
              <Form.Input
                field='WeChatPayReturnURL'
                label={t('支付完成返回地址（可选）')}
                placeholder={t('留空则返回到充值页')}
              />
            </Col>
          </Row>

          <Button onClick={submitWeChatPay} style={{ marginTop: 16 }}>
            {t('更新微信支付设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}

