import { test } from 'node:test';
import assert from 'node:assert/strict';
import { generateKeyPairSync, createHash, verify } from 'node:crypto';
import { mkdtemp, mkdir, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createSignedRelease } from './update-release.mjs';

const keys = generateKeyPairSync('rsa', { modulusLength: 2048 });
const config = { runtimeVersion: 'test-native-1', updates: { url: 'http://172.23.10.97:5174/v1/mobile/updates' } };
async function fixture() {
  const root = await mkdtemp(join(tmpdir(), 'broker-update-test-'));
  const exported = join(root, 'export'); await mkdir(exported);
  await writeFile(join(exported, 'app.hbc'), 'compiled Hermes bundle');
  await writeFile(join(exported, 'icon.png'), 'image bytes');
  await writeFile(join(exported, 'metadata.json'), JSON.stringify({ fileMetadata: { android: { bundle: 'app.hbc', assets: [{ path: 'icon.png', ext: 'png' }] } } }));
  return { exportDir: exported, outputDir: join(root, 'release'), config, version: '0.1.3', message: 'Test release', privateKey: keys.privateKey, certificate: keys.publicKey };
}

test('release manifest verifies against the app certificate and hashes every downloadable asset', async () => {
  const input = await fixture(); const manifest = await createSignedRelease(input);
  const envelope = JSON.parse(await readFile(join(input.outputDir, 'current/test-native-1/android.json'), 'utf8'));
  assert.ok(verify('RSA-SHA256', Buffer.from(JSON.stringify(envelope.manifest)), keys.publicKey, Buffer.from(envelope.signature, 'base64')));
  assert.ok(!verify('RSA-SHA256', Buffer.from(JSON.stringify({ ...envelope.manifest, runtimeVersion: 'wrong' })), keys.publicKey, Buffer.from(envelope.signature, 'base64')));
  for (const asset of [manifest.launchAsset, ...manifest.assets]) {
    assert.equal(new URL(asset.url).origin, 'http://172.23.10.97:5174');
    assert.match(new URL(asset.url).pathname, /^\/v1\/mobile\/assets\/[a-f0-9]{64}\./);
    const bytes = await readFile(join(input.outputDir, 'assets', new URL(asset.url).pathname.split('/').at(-1)));
    assert.equal(createHash('sha256').update(bytes).digest('base64url'), asset.hash);
  }
});

test('rejects a mismatched signing key and an export path outside its directory', async () => {
  const input = await fixture(); const other = generateKeyPairSync('rsa', { modulusLength: 2048 });
  await assert.rejects(createSignedRelease({ ...input, certificate: other.publicKey }), /does not match/);
  await writeFile(join(input.exportDir, 'metadata.json'), JSON.stringify({ fileMetadata: { android: { bundle: '../private-key.pem', assets: [] } } }));
  await assert.rejects(createSignedRelease(input), /escapes/);
});

test('incompatible runtime and a public HTTP update origin are refused before publication', async () => {
  const input = await fixture();
  await assert.rejects(createSignedRelease({ ...input, config: { ...config, runtimeVersion: '../other' } }), /runtimeVersion/);
  await assert.rejects(createSignedRelease({ ...input, config: { ...config, updates: { url: 'http://203.0.113.1/updates' } } }), /HTTPS/);
});
