// Local development only. No secret is printed or written to tracked files.
import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync, chmodSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const local = `${root}.local`;
const privateFile = (path, content) => {
  writeFileSync(path, content, { mode: 0o600 });
  chmodSync(path, 0o600);
};
try {
  const status = JSON.parse(execFileSync(`${root}node_modules/.bin/supabase`, ['status', '--output', 'json'], { cwd: root, stdio: ['ignore', 'pipe', 'pipe'] }));
  const authURL = new URL(status.API_URL);
  if (!['127.0.0.1', 'localhost'].includes(authURL.hostname) || authURL.port !== '54321') throw new Error('Nur die lokale Fitty-Instanz ist erlaubt.');
  mkdirSync(local, { recursive: true, mode: 0o700 });
  const accountFile = `${local}/account.json`;
  const account = existsSync(accountFile) ? JSON.parse(readFileSync(accountFile, 'utf8')) : {
    email: 'fitty@example.test', password: randomBytes(18).toString('base64url'), dbPassword: randomBytes(24).toString('hex'),
  };
  if (!/^[0-9a-f]{48}$/.test(account.dbPassword)) throw new Error('Ungültige lokale Datenbank-Konfiguration.');
  // Run migrations separately, so schema changes remain visible and reviewable.
  execFileSync('docker', ['exec', '-i', 'supabase_db_fitty', 'psql', '-U', 'postgres', '-d', 'postgres', '-v', 'ON_ERROR_STOP=1'], {
    input: `alter role fitty_api password '${account.dbPassword}';\n`, stdio: ['pipe', 'pipe', 'pipe'],
  });
  const login = await fetch(`${status.API_URL}/auth/v1/token?grant_type=password`, {
    method: 'POST', headers: { apikey: status.ANON_KEY, 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: account.email, password: account.password }),
  });
  if (!login.ok) {
    const created = await fetch(`${status.API_URL}/auth/v1/admin/users`, {
      method: 'POST', headers: { apikey: status.SERVICE_ROLE_KEY, Authorization: `Bearer ${status.SERVICE_ROLE_KEY}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: account.email, password: account.password, email_confirm: true }),
    });
    if (!created.ok) throw new Error('Entwicklungszugang konnte nicht angelegt werden. Vorhandenes Konto und .local/account.json prüfen.');
  }
  privateFile(accountFile, `${JSON.stringify(account, null, 2)}\n`);
  privateFile(`${local}/zugang.txt`, `Fitty – nur für die lokale Entwicklung\n\nAdresse: http://localhost:8788\nE-Mail: ${account.email}\nPasswort: ${account.password}\n`);
  const previousServerEnv = existsSync(`${root}server/.env`) ? readFileSync(`${root}server/.env`, 'utf8') : '';
  const aiConfig = previousServerEnv.split('\n').filter(line => /^(OPENAI_API_KEY|FITTY_OPENAI_MODEL)=/.test(line)).join('\n');
  privateFile(`${root}server/.env`, `FITTY_ADDR=127.0.0.1:8787\nFITTY_ALLOWED_ORIGINS=http://localhost:8788,http://127.0.0.1:8788\nFITTY_DATABASE_URL=postgresql://fitty_api:${account.dbPassword}@127.0.0.1:54322/postgres?sslmode=disable\nFITTY_SUPABASE_URL=${status.API_URL}\nFITTY_SUPABASE_KEY=${status.ANON_KEY}\nFITTY_SUPABASE_SERVICE_ROLE_KEY=${status.SERVICE_ROLE_KEY}\n${aiConfig || 'OPENAI_API_KEY=\nFITTY_OPENAI_MODEL=gpt-5-mini'}\n`);
  privateFile(`${root}apps/client/.env`, `EXPO_PUBLIC_API_URL=http://127.0.0.1:8787\nEXPO_PUBLIC_SUPABASE_URL=${status.API_URL}\nEXPO_PUBLIC_SUPABASE_KEY=${status.ANON_KEY}\n`);
  console.log('Lokaler Zugang eingerichtet. Anmeldedaten: .local/zugang.txt. API und Expo neu starten.');
} catch (error) {
  // Child-process errors can include SQL containing a password. Never print them.
  console.error(error?.status !== undefined ? 'Lokales Setup fehlgeschlagen. Supabase starten und Migrationen ausführen.' : error.message);
  process.exitCode = 1;
}
