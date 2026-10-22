import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
const root = fileURLToPath(new URL('../../', import.meta.url));
const compose = ['--context', 'default', 'compose', '-f', 'docker-compose.yml', '-f', 'mobile/backend/compose.yml'];
for (const args of [['up', '-d', 'postgres', 'redis'], ['up', '-d', '--no-deps', '--build', 'api']]) {
 const result = spawnSync('docker', [...compose, ...args], { cwd: root, stdio: 'inherit' });
 if (result.error) throw result.error;
 if (result.status !== 0) process.exit(result.status || 1);
}
console.log('Broker API: http://127.0.0.1:18080 / Wi-Fi http://192.168.20.96:18080; test KFB: localhost:8080; test trading enabled.');
