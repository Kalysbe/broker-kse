import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync, copyFileSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { spawn, spawnSync } from 'node:child_process';
import { phoneConfig } from './phone-config.mjs';

const action = process.argv[2] || 'run';
if (!['run', 'build', 'phone', 'start'].includes(action)) throw new Error('Use run, build, phone or start');
if (process.platform !== 'win32') throw new Error('Windows helper: use Expo CLI directly on other platforms.');
const source = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const mirror = resolve(process.env.ANDROID_BUILD_ROOT || `${process.env.SystemDrive || 'C:'}\\sbis-mobile`);
if (/[^\x00-\x7f]/.test(mirror) || mirror === source || mirror.startsWith(source + sep)) throw new Error('ANDROID_BUILD_ROOT must be a separate ASCII path.');
const java = process.env.JAVA_HOME || 'C:\\Program Files\\Android\\Android Studio\\jbr';
const sdk = process.env.ANDROID_HOME || join(process.env.LOCALAPPDATA, 'Android', 'Sdk');
if (!existsSync(join(java, 'bin', 'java.exe')) || !existsSync(join(sdk, 'platform-tools', 'adb.exe'))) throw new Error('Install Android Studio and SDK first; set JAVA_HOME / ANDROID_HOME for custom locations.');
const socketDir = join(process.env.USERPROFILE, '.android', 'sockets');
mkdirSync(socketDir, { recursive: true });
const env = { ...process.env, JAVA_HOME: java, ANDROID_HOME: sdk, JAVA_TOOL_OPTIONS: `${process.env.JAVA_TOOL_OPTIONS || ''} "-Djdk.net.unixdomain.tmpdir=${socketDir}"`, PATH: `${join(java, 'bin')};${join(sdk, 'platform-tools')};${process.env.PATH}`, EXPO_NO_TELEMETRY: '1' };
const phone = action === 'phone' ? phoneConfig(process.env.EXPO_PUBLIC_API_URL) : undefined;
if (action === 'phone') {
  env.EXPO_PUBLIC_DATA_SOURCE = 'kfb';
  env.EXPO_PUBLIC_API_URL = phone.apiURL;
  env.NODE_ENV = 'production';
}
function run(command, args, cwd = mirror) {
  const result = spawnSync(command, args, { cwd, env, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
const sourceExpo = join(source, 'node_modules', 'expo', 'bin', 'cli');
let ownedMetro;
process.on('exit', () => { if (ownedMetro && ownedMetro.exitCode === null) ownedMetro.kill(); });
async function metroReady() {
  try { return (await (await fetch('http://127.0.0.1:8081/status', { signal: AbortSignal.timeout(1500) })).text()).includes('packager-status:running'); } catch { return false; }
}
async function ensureMetro() {
  if (await metroReady()) return;
  ownedMetro = spawn(process.execPath, ['--dns-result-order=ipv4first', sourceExpo, 'start', '--localhost', '--port', '8081', '--max-workers', '2'], { cwd: source, env, stdio: 'inherit', windowsHide: true });
  let startupError;
  ownedMetro.once('error', error => { startupError = error; });
  const deadline = Date.now() + 40000;
  while (Date.now() < deadline) {
    if (startupError) throw startupError;
    if (ownedMetro.exitCode !== null) throw new Error('Metro exited before it became ready');
    if (await metroReady()) return;
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  throw new Error('Metro did not become ready on IPv4 port 8081');
}
if (action === 'start') run(process.execPath, ['--dns-result-order=ipv4first', sourceExpo, 'start', '--localhost', '--port', '8081', '--max-workers', '2'], source);
else {
  const marker = join(mirror, '.sbis-build-source.json');
  if (existsSync(mirror) && readdirSync(mirror).length && (!existsSync(marker) || JSON.parse(readFileSync(marker, 'utf8')).source !== source)) throw new Error('Build directory belongs to another project. Choose an empty ANDROID_BUILD_ROOT.');
  mkdirSync(mirror, { recursive: true });
  writeFileSync(marker, JSON.stringify({ source }));
  const lockHash = createHash('sha256').update(readFileSync(join(source, 'package-lock.json'))).digest('hex');
  const installed = join(mirror, '.sbis-lock-hash');
  const installNeeded = !existsSync(join(mirror, 'node_modules', 'expo')) || !existsSync(installed) || readFileSync(installed, 'utf8') !== lockHash;
  // Only generated copies of these two source directories are replaced. Validate
  // their resolved locations before recursive removal; never remove the workspace.
  for (const name of ['app', 'src']) {
    const target = resolve(mirror, name);
    if (!target.startsWith(mirror + sep) || resolve(source, name) === target) throw new Error('Unsafe build copy target');
    if (existsSync(target)) rmSync(target, { recursive: true });
    cpSync(join(source, name), target, { recursive: true });
  }
  for (const name of ['package.json', 'package-lock.json', 'app.json', 'tsconfig.json', 'babel.config.js', 'metro.config.js']) if (existsSync(join(source, name))) copyFileSync(join(source, name), join(mirror, name));
  console.log(`Native build copy: ${mirror}\nEditable source and Metro: ${source}`);
  if (installNeeded) {
    const npm = process.env.npm_execpath || join(dirname(process.execPath), 'node_modules', 'npm', 'bin', 'npm-cli.js');
    run(process.execPath, [npm, 'ci', '--prefer-offline', '--no-audit']);
    writeFileSync(installed, lockHash);
  }
  const expo = join(mirror, 'node_modules', 'expo', 'bin', 'cli');
  const nativeHash = createHash('sha256').update(lockHash).update(readFileSync(join(source, 'app.json'))).update(readFileSync(join(source, 'package.json'))).digest('hex');
  const nativeMarker = join(mirror, '.sbis-native-hash');
  if (!existsSync(join(mirror, 'android', 'gradlew.bat')) || !existsSync(nativeMarker) || readFileSync(nativeMarker, 'utf8') !== nativeHash) {
    run(process.execPath, [expo, 'prebuild', '--platform', 'android', '--no-install']);
    writeFileSync(nativeMarker, nativeHash);
  }
  copyFileSync(join(source, 'dev', 'android-native.gradle'), join(mirror, '.sbis-native.gradle'));
  const rootGradle = join(mirror, 'android', 'build.gradle');
  const nativeSettings = "apply from: '../.sbis-native.gradle'";
  // Register DSL callbacks before Expo's root plugin evaluates Android children.
  const previousRootScript = readFileSync(rootGradle, 'utf8');
  const rootScript = previousRootScript.split('\n').filter(line => line.trim() !== nativeSettings).join('\n');
  if (previousRootScript !== `${nativeSettings}\n${rootScript}`) writeFileSync(rootGradle, `${nativeSettings}\n${rootScript}`);
  if (action === 'build' || action === 'phone') {
    if (action === 'phone') {
      cpSync(join(source, 'dev', 'phone'), join(mirror, '.sbis-phone'), { recursive: true });
      writeFileSync(join(mirror, '.sbis-phone', 'res', 'xml', 'sbis_local_network.xml'), phone.networkSecurityXML);
    }
    const task = action === 'phone' ? ':app:assembleLocalPhone -PsbisPhone=true -PreactNativeArchitectures=arm64-v8a' : ':app:assembleDebug -PreactNativeArchitectures=x86_64';
    run('cmd.exe', ['/d', '/s', '/c', `gradlew.bat ${task} --no-daemon --max-workers=2 --console=plain`], join(mirror, 'android'));
    const output = join(source, 'artifacts'); mkdirSync(output, { recursive: true });
    const variant = action === 'phone' ? 'localPhone' : 'debug';
    const fileName = action === 'phone' ? 'kfb-invest-android.apk' : 'sbis-mobile-debug.apk';
    copyFileSync(join(mirror, 'android', 'app', 'build', 'outputs', 'apk', variant, `app-${variant}.apk`), join(output, fileName));
    console.log(`APK: ${join(output, fileName)} (${action === 'phone' ? `ARM64 phone; bundled interface; API ${phone.apiURL}` : 'x86_64 emulator, requires Metro'})`);
  } else {
    // A debug app needs its source bundle at first launch, before Activity starts.
    await ensureMetro();
    run(process.execPath, [expo, 'run:android', '--no-bundler']);
    if (ownedMetro) await new Promise(resolve => ownedMetro.once('exit', resolve));
    else console.log('Using the running Metro on port 8081.');
  }
}
