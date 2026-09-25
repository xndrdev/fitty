// Integration against local Supabase + running Go API. Temporary users and
// their cascaded profile/chat data are removed even when an assertion fails.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const config = JSON.parse(execFileSync(`${root}node_modules/.bin/supabase`, ['status', '--output', 'json'], { cwd: root, stdio: ['ignore', 'pipe', 'pipe'] }));
assert.equal(new URL(config.API_URL).hostname, '127.0.0.1', 'Only the local Supabase instance is allowed');
const users = [];
const adminHeaders = { apikey: config.SERVICE_ROLE_KEY, Authorization: `Bearer ${config.SERVICE_ROLE_KEY}`, 'Content-Type': 'application/json' };
async function newUser() {
  const account = { email: `fitty-test-${randomUUID()}@example.test`, password: randomBytes(20).toString('hex') };
  const response = await fetch(`${config.API_URL}/auth/v1/admin/users`, { method: 'POST', headers: adminHeaders, body: JSON.stringify({ ...account, email_confirm: true }) });
  if (!response.ok) throw new Error(`Test account setup failed: ${response.status} ${await response.text()}`);
  const user = await response.json(); users.push(user.id);
  const session = await fetch(`${config.API_URL}/auth/v1/token?grant_type=password`, { method: 'POST', headers: { apikey: config.ANON_KEY, 'Content-Type': 'application/json' }, body: JSON.stringify(account) });
  if (!session.ok) throw new Error(`Test login failed: ${session.status} ${await session.text()}`);
  return (await session.json()).access_token;
}
async function request(token, path, { method = 'GET', body, status = 200 } = {}) {
  const response = await fetch(`http://127.0.0.1:8787${path}`, { method, headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), 'Content-Type': 'application/json' }, ...(body ? { body: JSON.stringify(method === 'POST' && path.endsWith('/messages') ? { ...body, analyze: false } : body) } : {}) });
  const value = await response.json();
  assert.equal(response.status, status, `${method} ${path}: ${JSON.stringify(value)}`);
  return value;
}
try {
  const settings = await fetch(`${config.API_URL}/auth/v1/settings`, { headers: { apikey: config.ANON_KEY } });
  const authSettings = await settings.json();
  assert.equal(authSettings.disable_signup, true);
  assert.equal(authSettings.external.email, true);
  const first = await newUser(), second = await newUser();
  await request(null, '/v1/profile', { status: 401 });
  const profile = { display_name: 'Integrationstest', time_zone: 'Pacific/Auckland', goals: 'Regelmäßig bewegen', preferences: 'Vegetarisch' };
  await request(first, '/v1/profile', { method: 'PUT', body: profile });
  assert.deepEqual(await request(first, '/v1/profile'), profile);
  assert.equal((await request(second, '/v1/profile')).display_name, '');
  await request(first, '/v1/profile', { method: 'PUT', body: { ...profile, time_zone: 'Invalid/Zone' }, status: 400 });
  const body = { client_id: randomUUID(), content: 'Frühstück: Skyr und eine Banane.' };
  const path = '/v1/days/2026-09-16/messages';
  const saved = await request(first, path, { method: 'POST', body, status: 201 });
  assert.deepEqual(await request(first, path, { method: 'POST', body }), saved);
  await request(first, path, { method: 'POST', body: { ...body, content: 'Anderer Inhalt' }, status: 409 });
  await request(first, '/v1/days/2026-09-15/messages', { method: 'POST', body, status: 409 });
  assert.equal((await request(first, '/v1/days')).days.length, 1, 'Conflict must not leave an empty day');
  assert.equal((await request(second, path)).messages.length, 0);
  assert.equal((await request(second, '/v1/days')).days.length, 0);
  await request(second, path, { method: 'POST', body, status: 201 });
  await request(first, '/v1/days/2026-02-29/messages', { method: 'POST', body, status: 400 });
  await request(first, path, { method: 'POST', body: { client_id: randomUUID(), content: '  ' }, status: 400 });
  await request(first, path, { method: 'POST', body: { client_id: randomUUID(), content: 'a'.repeat(8001) }, status: 400 });
  // Concurrent retries share one record, including their returned message ID.
  const repeated = { client_id: randomUUID(), content: 'Walking Pad: 30 Minuten' };
  const responses = await Promise.all(Array.from({ length: 4 }, async () => {
    const res = await fetch(`http://127.0.0.1:8787${path}`, { method: 'POST', headers: { Authorization: `Bearer ${first}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ ...repeated, analyze: false }) });
    assert.ok([200, 201].includes(res.status)); return (await res.json()).message.id;
  }));
  assert.equal(new Set(responses).size, 1);
  for (let i = 0; i < 100; i++) await request(first, path, { method: 'POST', body: { client_id: randomUUID(), content: `Nachricht ${i}` }, status: 201 });
  const page = await request(first, path);
  assert.equal(page.messages.length, 100); assert.ok(page.next_before);
  const older = await request(first, `${path}?before=${page.next_before}`);
  assert.equal(older.messages.length, 2); assert.equal(older.next_before, null);
  assert.equal(new Set([...older.messages, ...page.messages].map(m => m.id)).size, 102);
  for (let day = 1; day <= 31; day++) await request(first, `/v1/days/2026-08-${String(day).padStart(2, '0')}/messages`, { method: 'POST', body: { client_id: randomUUID(), content: `Tag ${day}` }, status: 201 });
  const days = await request(first, '/v1/days'); assert.equal(days.days.length, 30);
  const olderDays = await request(first, `/v1/days?before=${days.next_before}`); assert.equal(olderDays.days.length, 2);
  console.log('OK: Anmeldung, Profil, Benutzertrennung, Kalendertage, Persistenz, parallele Wiederholungen und beide Verlaufspaginierungen.');
} finally {
  for (const id of users) {
    const response = await fetch(`${config.API_URL}/auth/v1/admin/users/${id}`, { method: 'DELETE', headers: adminHeaders });
    assert.ok(response.ok, 'Test account cleanup failed');
  }
}
