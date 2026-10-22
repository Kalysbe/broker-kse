import { test } from 'node:test';
import assert from 'node:assert/strict';
import { releaseInfo } from './release.ts';

test('embedded, offline and malformed update metadata retain the installed version', () => {
  for (const manifest of [null, {}, { extra: null }, { extra: { release: { version: 'not a version', message: 123 } } }]) {
    assert.deepEqual(releaseInfo(manifest, '0.1.2'), { version: '0.1.2', message: '' });
  }
});

test('a signed downloaded release supplies the displayed version and bounded notes', () => {
  assert.deepEqual(releaseInfo({ extra: { release: { version: '0.1.3', message: '  Улучшен вход  ' } } }, '0.1.2'), { version: '0.1.3', message: 'Улучшен вход' });
  assert.equal(releaseInfo({ extra: { release: { message: 'x'.repeat(3000) } } }, '0.1.2').message.length, 2000);
});
