// Local PostgreSQL integration with a fake AI provider. Only the local API's
// health status is read; the test process receives no OpenAI credentials.
import { execFileSync, spawn } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const users = [];
let config;
let failed = false;

function requireLocal(value, port) {
  const url = new URL(value);
  if (!['localhost', '127.0.0.1'].includes(url.hostname) || url.port !== port) {
    throw new Error('Tracking-Tests benötigen die lokale Fitty-Supabase-Instanz.');
  }
  return url;
}

async function requireOfflineLocalAPI() {
  try {
    await fetch('http://127.0.0.1:8787/healthz', {
      signal: AbortSignal.timeout(3000), redirect: 'error',
    });
  } catch (error) {
    // Connection refused establishes that no API is listening. A timeout or
    // other ambiguous failure must not hide a potentially active AI worker.
    if (error?.cause?.code === 'ECONNREFUSED') return;
    throw new Error('Status der lokalen API konnte nicht geprüft werden. Den lokalen Go-Server vor dem Offline-Trackingtest kurz stoppen.');
  }
  // Photo cleanup is another background worker even when AI is disabled.
  throw new Error('Den lokalen Go-Server vor test:tracking stoppen, damit KI-Worker und Upload-Bereinigung keine Testaufträge übernehmen.');
}

function adminHeaders() {
  return {
    apikey: config.SERVICE_ROLE_KEY,
    Authorization: `Bearer ${config.SERVICE_ROLE_KEY}`,
    'Content-Type': 'application/json',
  };
}

async function createUser() {
  const response = await fetch(`${config.API_URL}/auth/v1/admin/users`, {
    method: 'POST', headers: adminHeaders(), signal: AbortSignal.timeout(15000),
    body: JSON.stringify({
      email: `fitty-tracking-${randomUUID()}@example.test`,
      password: randomBytes(24).toString('hex'), email_confirm: true,
    }),
  });
  if (!response.ok) throw new Error(`Temporärer Tracking-Testzugang konnte nicht angelegt werden (HTTP ${response.status}).`);
  const user = await response.json();
  if (!/^[0-9a-f-]{36}$/i.test(user.id)) throw new Error('Ungültige Antwort beim Anlegen des Testzugangs.');
  users.push(user.id);
  return user.id;
}

try {
  try {
    config = JSON.parse(execFileSync(`${root}node_modules/.bin/supabase`, ['status', '--output', 'json'], {
      cwd: root, stdio: ['ignore', 'pipe', 'pipe'],
    }));
  } catch {
    throw new Error('Lokales Supabase ist nicht erreichbar. Keine Zugangsdaten wurden ausgegeben.');
  }
  requireLocal(config.API_URL, '54321');
  process.loadEnvFile(`${root}server/.env`);
  const database = requireLocal(process.env.FITTY_DATABASE_URL, '54322');
  if (database.username !== 'fitty_api' || database.pathname !== '/postgres') {
    throw new Error('Tracking-Tests benötigen die lokale eingeschränkte Datenbankrolle fitty_api.');
  }
  await requireOfflineLocalAPI();
  const userA = await createUser();
  const userB = await createUser();
  const environment = {
    ...process.env,
    FITTY_TEST_DATABASE_URL: process.env.FITTY_DATABASE_URL,
    FITTY_TEST_USER_A: userA, FITTY_TEST_USER_B: userB,
  };
  for (const name of Object.keys(environment)) {
    if (name.includes('OPENAI')) delete environment[name];
  }
  const exitCode = await new Promise((resolve, reject) => {
    const child = spawn('go', ['test', './internal/httpapi', '-run', '^TestTrackingIntegration$', '-count=1', '-timeout=3m', '-v'], {
      cwd: `${root}server`, env: environment, stdio: ['ignore', 'inherit', 'inherit'],
    });
    child.once('error', () => reject(new Error('Go-Integrationstest konnte nicht gestartet werden.')));
    child.once('exit', (code) => resolve(code ?? 1));
  });
  if (exitCode !== 0) throw new Error('Tracking-Integrationstest fehlgeschlagen. Testdetails stehen oben.');
} catch (error) {
  failed = true;
  // Never print child-process error objects, environment values or provider bodies.
  console.error(error instanceof Error && !('cmd' in error) ? error.message : 'Tracking-Test konnte nicht ausgeführt werden.');
} finally {
  for (const user of users) {
    try {
      const response = await fetch(`${config.API_URL}/auth/v1/admin/users/${user}`, {
        method: 'DELETE', headers: adminHeaders(), signal: AbortSignal.timeout(15000),
      });
      if (!response.ok) throw new Error('cleanup');
    } catch {
      failed = true;
      console.error(`Temporärer Tracking-Testzugang ${user} konnte nicht entfernt werden. Lokale Supabase-Verbindung prüfen.`);
    }
  }
}
if (failed) process.exitCode = 1;
else console.log('OK: Tracking, Korrekturen, Tageswerte, Nutzertrennung und Worker-Wiederholungen; temporäre Testdaten entfernt.');
