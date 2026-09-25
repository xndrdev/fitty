// Local Supabase Storage + Go integration. No OpenAI requests: each message
// explicitly sets analyze:false, even when the regular API worker is enabled.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import { deflateSync } from 'node:zlib';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const config = JSON.parse(execFileSync(`${root}node_modules/.bin/supabase`, ['status', '--output', 'json'], { cwd: root, stdio: ['ignore', 'pipe', 'pipe'] }));
assert.equal(config.API_URL, 'http://127.0.0.1:54321', 'Only local Supabase is allowed');
const admin = { apikey: config.SERVICE_ROLE_KEY, Authorization: `Bearer ${config.SERVICE_ROLE_KEY}`, 'Content-Type': 'application/json' };
const users = [], objects = new Set();
const date = '2026-09-16';
function testPNG(channel = 255) {
  function chunk(name, data) {
    const content = Buffer.concat([Buffer.from(name), data]);
    let crc = 0xffffffff;
    for (const byte of content) { crc ^= byte; for (let i = 0; i < 8; i++) crc = (crc >>> 1) ^ ((crc & 1) ? 0xedb88320 : 0); }
    const length = Buffer.alloc(4), checksum = Buffer.alloc(4);
    length.writeUInt32BE(data.length); checksum.writeUInt32BE((crc ^ 0xffffffff) >>> 0);
    return Buffer.concat([length, content, checksum]);
  }
  const dimensions = Buffer.alloc(13); dimensions.writeUInt32BE(2, 0); dimensions.writeUInt32BE(2, 4); dimensions[8] = 8; dimensions[9] = 6;
  const row = Buffer.from([0, channel, 0, 0, 255, channel, 0, 0, 255]);
  return Buffer.concat([Buffer.from('89504e470d0a1a0a', 'hex'), chunk('IHDR', dimensions), chunk('IDAT', deflateSync(Buffer.concat([row, row]))), chunk('IEND', Buffer.alloc(0))]);
}
async function newUser() {
  const account = { email: `fitty-photo-test-${randomUUID()}@example.test`, password: randomBytes(24).toString('hex') };
  const response = await fetch(`${config.API_URL}/auth/v1/admin/users`, { method: 'POST', headers: admin, body: JSON.stringify({ ...account, email_confirm: true }) });
  assert.ok(response.ok, `Test account: HTTP ${response.status}`);
  const user = await response.json(); users.push(user.id);
  const session = await fetch(`${config.API_URL}/auth/v1/token?grant_type=password`, { method: 'POST', headers: { apikey: config.ANON_KEY, 'Content-Type': 'application/json' }, body: JSON.stringify(account) });
  assert.ok(session.ok, 'Test login failed');
  return { id: user.id, token: (await session.json()).access_token };
}
async function request(user, path, { method = 'GET', body, image, status = 200 } = {}) {
  const response = await fetch(`http://127.0.0.1:8787${path}`, { method, headers: { Authorization: `Bearer ${user.token}`, 'Content-Type': image ? 'image/png' : 'application/json' }, body: image ?? (body ? JSON.stringify(body) : undefined), signal: AbortSignal.timeout(70000) });
  const value = await response.json();
  assert.equal(response.status, status, `${method} ${path}: ${JSON.stringify(value)}`);
  return value;
}
async function upload(user, id, image = testPNG(), status = 200, day = date) {
  // Reserve our own cleanup path even if the upload response is lost.
  objects.add(`${user.id}/${id}.jpg`);
  return request(user, `/v1/days/${day}/attachments/${id}`, { method: 'PUT', image, status });
}
try {
  const a = await newUser(), b = await newUser();
  assert.equal((await request(a, `/v1/days/${date}/summary`)).photos_enabled, true);
  const id = randomUUID();
  const first = await upload(a, id);
  assert.equal(first.attachment.mime_type, 'image/jpeg');
  assert.equal(first.attachment.width, 2);
  assert.deepEqual(await upload(a, id), first);
  await upload(a, id, testPNG(0), 409);
  await upload(b, id, testPNG(), 409);
  await upload(a, id, testPNG(), 409, '2026-09-15');
  await request(b, `/v1/attachments/${id}/url`, { status: 404 });
  const signed = await request(a, `/v1/attachments/${id}/url`);
  assert.ok(signed.path.startsWith('/storage/v1/object/sign/chat-attachments/'));
  const image = await fetch(`${config.API_URL}${signed.path}`);
  assert.ok(image.ok); assert.equal(image.headers.get('content-type'), 'image/jpeg');
  const privateRead = await fetch(`${config.API_URL}/storage/v1/object/chat-attachments/${a.id}/${id}.jpg`, { headers: { apikey: config.ANON_KEY, Authorization: `Bearer ${b.token}` } });
  assert.equal(privateRead.ok, false, 'Other users must not read private objects directly');
  const publicRead = await fetch(`${config.API_URL}/storage/v1/object/public/chat-attachments/${a.id}/${id}.jpg`);
  assert.equal(publicRead.ok, false, 'The photo bucket must remain private');
  const body = { client_id: randomUUID(), content: '', attachment_ids: [id], analyze: false };
  const path = `/v1/days/${date}/messages`;
  await request(b, path, { method: 'POST', body, status: 409 });
  const saved = await request(a, path, { method: 'POST', body, status: 201 });
  assert.equal(saved.message.analysis_status, 'disabled');
  assert.equal(saved.message.attachments[0].id, id);
  assert.deepEqual(await request(a, path, { method: 'POST', body }), saved);
  await request(a, path, { method: 'POST', body: { ...body, content: 'Different', attachment_ids: [] }, status: 409 });
  await request(a, path, { method: 'POST', body: { ...body, client_id: randomUUID() }, status: 409 });
  await request(a, `/v1/attachments/${id}`, { method: 'DELETE', status: 409 });
  const loaded = await request(a, path);
  assert.equal(loaded.messages.length, 1); assert.deepEqual(loaded.messages[0].attachments, saved.message.attachments);
  const removeID = randomUUID(); await upload(a, removeID);
  await request(a, `/v1/attachments/${removeID}`, { method: 'DELETE' });
  await request(a, `/v1/attachments/${removeID}`, { method: 'DELETE' });
  await request(a, `/v1/attachments/${removeID}/url`, { status: 404 });
  await upload(a, randomUUID(), Buffer.from('not an image'), 400);
  await request(a, path, { method: 'POST', body: { ...body, client_id: randomUUID(), attachment_ids: Array.from({ length: 5 }, () => randomUUID()) }, status: 400 });
  console.log('OK: Privater Storage, Bildaufbereitung, Zugriff, Upload-Wiederholung, reine Bildnachricht, Tages-/Nutzerzuordnung, Persistenz und Entfernen ohne KI-Aufrufe.');
} finally {
  if (objects.size) {
    const result = await fetch(`${config.API_URL}/storage/v1/object/chat-attachments`, { method: 'DELETE', headers: admin, body: JSON.stringify({ prefixes: [...objects] }) });
    assert.ok(result.ok, 'Test objects could not be removed');
  }
  for (const user of users) {
    const result = await fetch(`${config.API_URL}/auth/v1/admin/users/${user}`, { method: 'DELETE', headers: admin });
    assert.ok(result.ok, 'Test account could not be removed');
  }
}
