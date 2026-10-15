// Local-only Ark simulator. No external API requests or real provider keys.
// Run: node scripts/seedance25-local-mock.mjs
import http from 'node:http';
import { randomUUID } from 'node:crypto';
const tasks = new Map();
const server = http.createServer(async (req, res) => {
  res.setHeader('Content-Type', 'application/json');
  const send = (body, status = 200) => { res.statusCode = status; res.end(JSON.stringify(body)); };
  if (req.method === 'POST' && req.url === '/api/v3/contents/generations/tasks') {
    try {
      let raw = ''; for await (const part of req) raw += part;
      const body = JSON.parse(raw);
      const text = (body.content || []).filter((x) => x.type === 'text').map((x) => x.text).join(' ');
      const id = `local-video-${randomUUID()}`;
      const status = text.includes('[fail]') ? 'failed' : text.includes('[cancel]') ? 'cancelled' : 'succeeded';
      tasks.set(id, {
        id, model: body.model, status,
        resolution: body.draft ? '480p' : body.resolution || '720p',
        duration: 4.6, framespersecond: 24,
        usage: text.includes('[missing]') ? {} : { completion_tokens: 100000, total_tokens: 100000 },
        content: { video_url: 'http://127.0.0.1:13882/mock-no-real-video.mp4' },
        error: status === 'succeeded' ? undefined : { code: 'LocalMock', message: '本地模拟失败或取消' },
      });
      send({ id });
    } catch { send({ error: { message: 'Invalid local mock request' } }, 400); }
  } else if (req.method === 'GET' && req.url.startsWith('/api/v3/contents/generations/tasks/')) {
    const task = tasks.get(req.url.split('/').at(-1));
    send(task || { error: { message: 'Local task not found' } }, task ? 200 : 404);
  } else send({ error: { message: 'Local mock endpoint only' } }, 404);
});
server.listen(13882, '127.0.0.1', () => console.log('Local Ark mock: http://127.0.0.1:13882 (no paid requests)'));
