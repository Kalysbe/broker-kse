export type ReleaseInfo = { version: string; message: string };

export function releaseInfo(manifest: unknown, fallbackVersion: string): ReleaseInfo {
  const extra = manifest && typeof manifest === 'object' && 'extra' in manifest ? manifest.extra : null;
  const release = extra && typeof extra === 'object' && 'release' in extra ? extra.release : null;
  const version = release && typeof release === 'object' && 'version' in release && typeof release.version === 'string' && /^\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.-]+)?$/.test(release.version) ? release.version : fallbackVersion;
  const message = release && typeof release === 'object' && 'message' in release && typeof release.message === 'string' ? release.message.trim().slice(0, 2000) : '';
  return { version, message };
}
