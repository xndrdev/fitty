import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';

const envFile = new URL('../server/.env', import.meta.url);
if (existsSync(envFile)) process.loadEnvFile(envFile);
const child = spawn('go', ['run', './cmd/api'], {
  cwd: new URL('../server/', import.meta.url), stdio: 'inherit', env: process.env,
});
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal));
child.on('error', () => { console.error('Go konnte nicht gestartet werden.'); process.exitCode = 1; });
child.on('exit', code => { process.exitCode = code ?? 0; });
