import { createHash, randomUUID, sign, verify } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { isAbsolute, join, relative, resolve } from 'node:path';
import { phoneConfig } from './phone-config.mjs';

const mime = { png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', webp: 'image/webp', gif: 'image/gif', ttf: 'font/ttf', otf: 'font/otf', woff: 'font/woff', woff2: 'font/woff2', json: 'application/json', mp3: 'audio/mpeg', mp4: 'video/mp4' };

export async function createSignedRelease({ exportDir, outputDir, config, platform = 'android', version, message, privateKey, certificate, createdAt = new Date().toISOString(), id = randomUUID() }) {
  if (!['android', 'ios'].includes(platform) || !/^[A-Za-z0-9][A-Za-z0-9_-]{0,95}$/.test(config.runtimeVersion || '')) throw new Error('A compatible explicit runtimeVersion is required.');
  if (!/^\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.-]+)?$/.test(version || '')) throw new Error('Supply a semantic release version.');
  const updateURL = new URL(config.updates.url);
  phoneConfig(updateURL.origin);
  if (updateURL.username || updateURL.password || updateURL.search || updateURL.hash) throw new Error('Update URL must not contain credentials or query parameters.');
  const challenge = Buffer.from('broker-mobile-release-key-check');
  if (!verify('RSA-SHA256', challenge, certificate, sign('RSA-SHA256', challenge, privateKey))) throw new Error('Signing key does not match the certificate embedded in the app.');

  const exported = resolve(exportDir), output = resolve(outputDir);
  const metadata = JSON.parse(await readFile(join(exported, 'metadata.json'), 'utf8'));
  const files = metadata.fileMetadata?.[platform];
  if (!files?.bundle || !Array.isArray(files.assets)) throw new Error('Missing Expo export metadata for platform.');
  await mkdir(join(output, 'assets'), { recursive: true });
  async function asset(file, extension, launch = false) {
    if (!/^[a-z0-9]{1,10}$/.test(extension)) throw new Error('Unsupported asset extension.');
    const source = resolve(exported, file), location = relative(exported, source);
    if (!location || location.startsWith('..') || isAbsolute(location)) throw new Error('Export asset escapes the export directory.');
    const bytes = await readFile(source);
    const digest = createHash('sha256').update(bytes).digest();
    const name = `${digest.toString('hex')}.${extension}`;
    try { await writeFile(join(output, 'assets', name), bytes, { flag: 'wx' }); }
    catch (error) { if (error.code !== 'EEXIST') throw error; }
    return { hash: digest.toString('base64url'), key: createHash('md5').update(bytes).digest('hex'), fileExtension: `.${extension}`, contentType: launch ? 'application/javascript' : mime[extension] || 'application/octet-stream', url: new URL(`./assets/${name}`, updateURL).href };
  }
  const assets = [];
  for (const item of files.assets) assets.push(await asset(item.path, item.ext));
  const manifest = { id, createdAt, runtimeVersion: config.runtimeVersion, launchAsset: await asset(files.bundle, 'bundle', true), assets, metadata: {}, extra: { expoClient: config, release: { version, message: String(message || '').trim().slice(0, 2000) } } };
  const manifestBytes = Buffer.from(JSON.stringify(manifest));
  const signature = sign('RSA-SHA256', manifestBytes, privateKey).toString('base64');
  const envelope = JSON.stringify({ manifest, signature });
  const currentDirectory = join(output, 'current', config.runtimeVersion);
  await mkdir(currentDirectory, { recursive: true });
  await mkdir(join(output, 'releases'), { recursive: true });
  await writeFile(join(output, 'releases', `${id}.json`), envelope);
  await writeFile(join(currentDirectory, `${platform}.json`), envelope);
  return manifest;
}
