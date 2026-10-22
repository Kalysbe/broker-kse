import { spawnSync } from 'node:child_process';
const result = spawnSync('docker', ['compose', '-f', 'dev/compose.yml', 'up', '-d', '--build', 'api', 'worker'], { stdio: 'inherit' });
if (result.status !== 0) process.exit(result.status || 1);
// Readiness is checked by setup; this process only targets the dedicated loopback dev server.
for (let attempt = 0; attempt < 30; attempt++) {
  try { const response = await fetch('http://127.0.0.1:18080/health/ready'); if (response.ok && (await response.json()).ready) break; } catch {}
  if (attempt === 29) throw new Error('Local API did not become ready');
  await new Promise(resolve => setTimeout(resolve, 1000));
}
await import('./setup.mjs');
