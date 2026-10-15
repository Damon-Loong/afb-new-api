// Only initializes a fresh loopback preview. Never point this at a real site.
// LOCAL_VIDEO_ADMIN_PASSWORD must be provided; it is not stored in this script.
const base = 'http://127.0.0.1:13881';
const password = process.env.LOCAL_VIDEO_ADMIN_PASSWORD;
if (!password) throw new Error('Set LOCAL_VIDEO_ADMIN_PASSWORD for the isolated preview');
let cookie = '';
let userID = '';
async function api(path, method = 'GET', body) {
  const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', Cookie: cookie, ...(userID ? { 'New-Api-User': userID } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) });
  if (res.headers.get('set-cookie')) cookie = res.headers.get('set-cookie').split(';')[0];
  const result = await res.json();
  if (!res.ok || result.success === false) throw new Error(`${path}: ${result.message || res.status}`);
  return result.data;
}
const setup = await api('/api/setup');
if (setup.status || setup.root_init) throw new Error('Refusing to overwrite an initialized database; use the existing preview login');
await api('/api/setup', 'POST', { username: 'video_admin', password, confirmPassword: password, SelfUseModeEnabled: false, DemoSiteEnabled: false });
const user = await api('/api/user/login', 'POST', { username: 'video_admin', password });
userID = String(user.id);
const model = 'doubao-seedance-2-5-260628';
const pricing = { exchange_rate: 7, low_no_video: 70, low_with_video: 42, high_no_video: 77, high_with_video: 46, reserve_tokens: 300000 };
for (const [key, value] of Object.entries({
  'billing_setting.video_pricing': { [model]: pricing },
  'billing_setting.billing_mode': { [model]: 'video_tokens' },
  GroupRatio: { default: 1, vip: 0.6 },
  'general_setting.quota_display_type': 'CNY',
  USDExchangeRate: '7',
})) {
  await api('/api/option/', 'PUT', { key, value: typeof value === 'string' ? value : JSON.stringify(value) });
}
await api('/api/channel/', 'POST', { mode: 'single', channel: { name: 'Seedance 2.5 本地模拟（不调用真实上游）', type: 54, key: 'local-mock-not-a-provider-key', base_url: 'http://127.0.0.1:13882', models: model, group: 'default,vip', status: 1 } });
await api('/api/token/', 'POST', { name: 'Seedance 本地测试', remain_quota: 90000000, expired_time: -1, group: 'default', unlimited_quota: false });
console.log('Preview initialized: video_admin. Pricing and mock channel only; no production data.');
