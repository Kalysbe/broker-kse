import { test } from 'node:test';
import assert from 'node:assert/strict';
import { phoneConfig } from './phone-config.mjs';
import appConfig from '../app.config.js';
import { readFileSync } from 'node:fs';

test('phone build and Android HTTP allowlist target the deployed server', () => {
  const config = phoneConfig();
  assert.equal(config.apiURL, 'http://172.23.10.97:5174');
  assert.match(config.networkSecurityXML, /<domain includeSubdomains="false">172\.23\.10\.97<\/domain>/);
  assert.match(config.networkSecurityXML, /<base-config cleartextTrafficPermitted="false"/);
  assert.ok(!config.networkSecurityXML.includes('192.168.20.96'));
});

test('Expo updates use the direct backend route on the configured API port', () => {
  const previous = process.env.EXPO_PUBLIC_API_URL;
  const config = JSON.parse(readFileSync(new URL('../app.json', import.meta.url), 'utf8')).expo;
  try {
    delete process.env.EXPO_PUBLIC_API_URL;
    assert.equal(appConfig({ config }).updates.url, 'http://172.23.10.97:5174/v1/mobile/updates');
    process.env.EXPO_PUBLIC_API_URL = 'http://192.168.20.96:18080/';
    assert.equal(appConfig({ config }).updates.url, 'http://192.168.20.96:18080/v1/mobile/updates');
  } finally {
    if (previous === undefined) delete process.env.EXPO_PUBLIC_API_URL;
    else process.env.EXPO_PUBLIC_API_URL = previous;
  }
});

test('an explicit build origin updates the API and its exact Android allowlist', () => {
  const config = phoneConfig('http://192.168.20.96:18080/');
  assert.equal(config.apiURL, 'http://192.168.20.96:18080');
  assert.match(config.networkSecurityXML, />192\.168\.20\.96<\/domain>/);
  assert.ok(!config.networkSecurityXML.includes('172.23.10.97'));
});

test('HTTPS builds do not allow any cleartext domain', () => {
  const config = phoneConfig('https://broker.example');
  assert.equal(config.apiURL, 'https://broker.example');
  assert.ok(!config.networkSecurityXML.includes('domain-config'));
});

test('rejects public HTTP, credentials and URLs that are not origins', () => {
  for (const value of ['http://203.0.113.20', 'http://broker.example', 'https://name:secret@broker.example', 'https://broker.example/api', 'https://broker.example?token=secret', 'file:///tmp/x']) {
    assert.throws(() => phoneConfig(value));
  }
});
