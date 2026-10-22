import { readFile } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import { spawnSync } from 'node:child_process';
import { createSignedRelease } from './update-release.mjs';
import { phoneConfig } from './phone-config.mjs';

const source = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const { values } = parseArgs({ options: { version: { type: 'string' }, message: { type: 'string' }, 'export-dir': { type: 'string' }, output: { type: 'string' }, key: { type: 'string' }, 'skip-export': { type: 'boolean', default: false } } });
if (!values.version) throw new Error('Use --version 0.1.3 --message "Release notes".');
const exportDir = resolve(source, values['export-dir'] || '../.deploy/mobile-export');
const outputDir = resolve(source, values.output || '../.deploy/mobile-update-release');
const env = { ...process.env, EXPO_PUBLIC_API_URL: phoneConfig(process.env.EXPO_PUBLIC_API_URL).apiURL, EXPO_PUBLIC_DATA_SOURCE: 'kfb', NODE_ENV: 'production', EXPO_NO_TELEMETRY: '1' };
const expo = join(source, 'node_modules', 'expo', 'bin', 'cli');
function run(args, capture = false) {
  const result = spawnSync(process.execPath, [expo, ...args], { cwd: source, env, stdio: capture ? ['ignore', 'pipe', 'inherit'] : 'inherit', encoding: 'utf8' });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Expo command failed (${result.status}).`);
  return result.stdout;
}
if (!values['skip-export']) run(['export', '--platform', 'android', '--output-dir', exportDir, '--max-workers', '2']);
const config = JSON.parse(run(['config', '--type', 'public', '--json'], true));
const appConfig = JSON.parse(await readFile(join(source, 'app.json'), 'utf8')).expo;
const manifest = await createSignedRelease({ exportDir, outputDir, config, version: values.version, message: values.message, privateKey: await readFile(resolve(source, values.key || '../.deploy/update-signing/private-key.pem')), certificate: await readFile(resolve(source, appConfig.updates.codeSigningCertificate)) });
console.log(`Signed update ${manifest.extra.release.version}: ${manifest.id}\nRuntime: ${manifest.runtimeVersion}\nRelease directory: ${outputDir}\nUpload assets and release history first, then atomically replace current/${manifest.runtimeVersion}/android.json. Never upload the signing key.`);
