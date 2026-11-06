import React, { useEffect, useState } from 'react';
import { Button, Card, Switch, Input, TextArea, InputNumber, Typography, Banner, Spin, Space } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

export default function AccountSecuritySetting({ userId = 0 }) {
  const [policy, setPolicy] = useState(null);
  const [events, setEvents] = useState([]);
  const [busy, setBusy] = useState(false);
  const [loadError, setLoadError] = useState('');
  const global = !userId;
  useEffect(() => {
    let active = true;
    setPolicy(null); setLoadError('');
    API.get(`/api/account-security/${userId}`).then(({ data }) => {
      if (!active) return;
      if (!data.success) throw new Error(data.message);
      setPolicy(data.data.policy); setEvents(data.data.events || []);
    }).catch((e) => { if (active) setLoadError(e.message); });
    return () => { active = false; };
  }, [userId]);
  const update = (key, value) => setPolicy((p) => ({ ...p, [key]: value }));
  const rule = (key, value) => setPolicy((p) => ({ ...p, rules: { ...p.rules, [key]: value } }));
  const save = async () => {
    setBusy(true);
    try {
      const { data } = await API.put(`/api/account-security/${userId}`, policy);
      if (!data.success) throw new Error(data.message);
      showSuccess('安全策略已保存');
    } catch (e) { showError(e.message); } finally { setBusy(false); }
  };
  const field = (label, child, hint) => <div style={{ margin: '14px 0' }}><Text strong>{label}</Text><div style={{ marginTop: 6 }}>{child}</div>{hint && <div><Text type='tertiary' size='small'>{hint}</Text></div>}</div>;
  const number = (key, min = 0, max = 1000000000) => <InputNumber aria-label={key} value={policy.rules[key]} min={min} max={max} onChange={(v) => rule(key, v ?? 0)} style={{ width: 200 }} />;
  if (loadError) return <Banner type='danger' description={`无法读取账户安全策略：${loadError}`} />;
  if (!policy) return <Spin />;
  const rules = policy.rules;
  return <Card style={{ marginTop: 16 }}>
    <Title heading={4}>{global ? '账户安全与管理员告警' : '该用户的登录与告警策略'}</Title>
    <Banner type='info' style={{ marginTop: 12 }} description='默认不启用限制和告警。金额使用平台内部美元额度（USD），不是人民币；只告警、不自动封禁。已受理任务仍按原流程结算。' />
    {!global && <>
      {field('仅保留一个网页登录会话', <Switch aria-label='单会话登录' checked={policy.single_session} onChange={(v) => update('single_session', v)} />, '新登录使旧会话失效；多标签页不受影响。开启后旧会话可能需要重新登录。不影响模型API令牌；管理访问令牌不能绕过此限制。')}
      {field('允许登录的IP / 网段', <TextArea aria-label='登录IP白名单' value={policy.login_ips} onChange={(v) => update('login_ips', v)} placeholder='每行一个IP或CIDR网段；留空不限制' />, '限制登录和后续后台访问。动态IP用户请谨慎开启；被锁定后由其他管理员解除。')}
      {field('登录邮件通知用户', <Switch aria-label='登录邮件通知' checked={policy.login_notify} onChange={(v) => update('login_notify', v)} />, '发到用户绑定邮箱。同IP和浏览器24小时内最多一次；不包含令牌或密码。')}
      {field('单独设置该用户的告警阈值', <Switch aria-label='独立告警阈值' checked={policy.override_alerts} onChange={(v) => update('override_alerts', v)} />, '关闭时继承全局告警策略。通知收件人统一由超级管理员设置。')}
    </>}
    {global && <>
      {field('管理员收件邮箱', <TextArea aria-label='管理员告警邮箱' value={policy.admin_emails} onChange={(v) => update('admin_emails', v)} placeholder='每行一个邮箱，最多10个' />, '用量和余额告警均发送到此处；请先配置系统SMTP。保存不会发送测试邮件。')}
      {field('余额告警短信手机号', <TextArea aria-label='管理员短信手机号' value={policy.admin_phones} onChange={(v) => update('admin_phones', v)} placeholder='可选，每行一个大陆手机号' />)}
      {field('余额告警短信模板', <Input aria-label='短信通知模板' value={policy.sms_template} onChange={(v) => update('sms_template', v)} placeholder='SMS_...' />, '使用现有阿里云短信凭据、签名，另配通知类模板，变量为 ${user}。请勿填登录验证码模板；短信会产生运营商费用。')}
      {field('可信反向代理IP / 网段', <TextArea aria-label='可信代理网段' value={policy.trusted_proxies} onChange={(v) => update('trusted_proxies', v)} placeholder='仅填写实际部署的代理IP或CIDR；默认不信任转发头' />, '只由部署人员配置。留空使用连接来源IP；有代理时未正确配置会把代理IP当用户IP，不要填写全网网段。')}
    </>}
    {(global || policy.override_alerts) && <>
      <Title heading={5} style={{ marginTop: 24 }}>余额提醒</Title>
      {field('开启管理员低余额提醒', <Switch aria-label='低余额提醒' checked={rules.low_enabled} onChange={(v) => rule('low_enabled', v)} />)}
      {field('普通提醒阈值（USD）', number('low_usd'))}
      {field('紧急提醒阈值（USD）', number('critical_usd'), '须不高于普通阈值；余额恢复后重新进入低余额区间会再次提醒。现有余额不足拒绝新请求的规则保持不变。')}
      <Title heading={5} style={{ marginTop: 24 }}>用量激增提醒</Title>
      {field('开启用量告警邮件', <Switch aria-label='用量告警' checked={rules.usage_enabled} onChange={(v) => rule('usage_enabled', v)} />)}
      {field('监测窗口（分钟）', number('window_minutes', 1, 1440))}
      {field('窗口消费阈值（USD额度，0关闭）', number('spend_usd'))}
      {field('窗口请求记录数阈值（0关闭）', number('requests'))}
      {field('历史激增倍数（0关闭）', number('spike_ratio'), '与前7天同时段平均值比较，历史为零时仅使用固定阈值。视频仅在结算时计一次，不把预扣计作消费。')}
      {field('历史比较最低消费（USD）', number('spike_min_usd'))}
      {field('重复通知间隔（分钟）', number('cooldown_minutes', 5, 10080), '每分钟监测；紧急余额级别可立即升级提醒。此处不会自动停用账号。')}
    </>}
    <Space style={{ margin: '16px 0' }}><Button theme='solid' type='primary' loading={busy} onClick={save}>保存安全策略</Button><Text type='tertiary'>与用户基本信息分开保存</Text></Space>
    <Title heading={6}>最近30条安全记录（保留90天）</Title>
    {!events.length && <Text type='tertiary'>暂无记录；通知失败会记录 failed，请检查邮件或短信配置。</Text>}
    {events.map((e) => <div key={e.id} style={{ padding: '10px 0', borderBottom: '1px solid var(--semi-color-border)' }}><Text type='tertiary'>{new Date(e.created_at * 1000).toLocaleString()} · {e.kind} · {e.delivery}</Text><div>{e.message}</div></div>)}
  </Card>;
}
