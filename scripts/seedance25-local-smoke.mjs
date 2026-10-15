// End-to-end smoke test against the isolated loopback demo and local mock only.
import assert from 'node:assert/strict';
const base = 'http://127.0.0.1:13881';
const model = 'doubao-seedance-2-5-260628';
let cookie = '', userID = '', token = '';
async function request(path, method = 'GET', body, relay = false) {
  const res = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', ...(relay ? { Authorization: `Bearer ${token}` } : { Cookie: cookie, ...(userID ? { 'New-Api-User': userID } : {}) }) }, body: body === undefined ? undefined : JSON.stringify(body) });
  if (!relay && res.headers.get('set-cookie')) cookie = res.headers.get('set-cookie').split(';')[0];
  const data = await res.json();
  assert.ok(res.ok && data.success !== false && !data.error, `${path}: ${JSON.stringify(data)}`);
  return data;
}
const login = await request('/api/user/login', 'POST', { username: 'video_admin', password: process.env.LOCAL_VIDEO_ADMIN_PASSWORD });
userID = String(login.data.id);
// Refuse accidental use of a non-mock channel even on localhost.
const channels = (await request('/api/channel/')).data;
const channelList = channels.items || channels;
assert.ok(Array.isArray(channelList) && channelList.length === 1 && channelList[0].base_url === 'http://127.0.0.1:13882', 'Only the local mock channel is permitted');
const tokens = (await request('/api/token/')).data.items;
token = (await request(`/api/token/${tokens[0].id}/key`, 'POST')).data.key;
const quotaUnit = (await request('/api/status')).data.quota_per_unit;
const before = (await request('/api/user/self')).data.quota;
const cases = [];
async function submit(name, resolution, video, rate, marker = '', draft = false, draftID = '') {
  const content = draftID ? [{ type: 'draft_task', draft_task: { id: draftID } }] : [{ type: 'text', text: `local test ${name} ${marker}` }, ...(video ? [{ type: 'video_url', video_url: { url: 'http://127.0.0.1:13882/reference.mp4' }, role: 'reference_video' }] : [])];
  const response = await request('/api/v3/contents/generations/tasks', 'POST', { model, content, resolution, duration: 5, draft }, true);
  const pending = marker === '[missing]';
  const expected = marker === '[fail]' || marker === '[cancel]' ? 0 : Math.round((pending ? 0.3 : 0.1) * rate / 7 * quotaUnit);
  cases.push({ name, id: response.id, expected, pending });
  return response.id;
}
await submit('720p-text', '720p', false, 70);
await submit('720p-video', '720p', true, 42);
await submit('1080p-text', '1080p', false, 77);
await submit('1080p-video', '1080p', true, 46);
await submit('failure-refund', '720p', false, 70, '[fail]');
await submit('cancel-refund', '720p', false, 70, '[cancel]');
await submit('missing-usage-pending', '720p', false, 70, '[missing]');
const draftID = await submit('draft-video-input', '480p', true, 42, '', true);
async function verify() {
  for (let i = 0; i < 25; i++) {
    const tasks = (await request('/api/task/self?page_size=100')).data.items;
    if (cases.every((c) => tasks.some((task) => task.task_id === c.id && (c.pending ? task.properties?.billing_status === 'pending' : ['SUCCESS', 'FAILURE'].includes(task.status))))) {
      for (const c of cases) {
        const task = tasks.find((t) => t.task_id === c.id);
        assert.equal(task.quota, c.expected, `${c.name} quota`);
      }
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 2000));
  }
  throw new Error('Tasks did not reach expected billing states within 50 seconds');
}
await verify();
await submit('draft-final-inherits-video-input', '1080p', true, 46, '', false, draftID);
// Also exercise the converted /v1/videos path.
const converted = await request('/v1/videos', 'POST', { model, prompt: 'local converted test', seconds: '5', metadata: { resolution: '720p' } }, true);
cases.push({ name: 'converted-video-api', id: converted.id, expected: Math.round(0.1 * 70 / 7 * quotaUnit), pending: false });
await verify();
const after = (await request('/api/user/self')).data.quota;
assert.equal(before - after, cases.reduce((sum, c) => sum + c.expected, 0), 'wallet matches actual settled + pending reserved quota');
console.log(JSON.stringify({ passed: cases.length, cases: cases.map(({ name, expected, pending }) => ({ name, expectedQuota: expected, pending })), walletReconciled: true }, null, 2));
