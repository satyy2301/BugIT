'use strict';

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const extRoot = path.join(__dirname, '..');
const pkg = JSON.parse(fs.readFileSync(path.join(extRoot, 'package.json'), 'utf8'));
const expectedVersion = pkg.version;

const platforms = [
  { dir: 'win32-x64', exe: 'bugit.exe' },
  { dir: 'linux-x64', exe: 'bugit' },
  { dir: 'darwin-arm64', exe: 'bugit' },
];

function run(bin, args) {
  return spawnSync(bin, args, { encoding: 'utf8', timeout: 15000 });
}

let failed = false;
let verified = 0;
const requireAll = process.env.BUGIT_VERIFY_ALL_PLATFORMS === '1';

for (const { dir, exe } of platforms) {
  const binPath = path.join(extRoot, 'bin', dir, exe);
  if (!fs.existsSync(binPath)) {
    if (requireAll) {
      console.error(`MISSING: ${binPath}`);
      failed = true;
    } else {
      console.log(`SKIP: ${dir}/${exe} (not bundled on this platform)`);
    }
    continue;
  }

  const versionResult = run(binPath, ['version']);
  if (versionResult.status !== 0) {
    console.error(`FAIL: ${binPath} version exited ${versionResult.status}`);
    console.error(versionResult.stderr || versionResult.stdout);
    failed = true;
    continue;
  }

  const versionLine = (versionResult.stdout || '').trim();
  const match = versionLine.match(/(\d+\.\d+\.\d+)/);
  const binVersion = match ? match[1] : '';
  if (binVersion !== expectedVersion) {
    console.error(`FAIL: ${binPath} reports ${binVersion || versionLine}, expected ${expectedVersion}`);
    failed = true;
    continue;
  }

  const recordHelp = run(binPath, ['record', '--help']);
  const helpText = `${recordHelp.stdout || ''}${recordHelp.stderr || ''}`;
  if (!helpText.includes('record')) {
    console.error(`FAIL: ${binPath} does not support "record" subcommand`);
    failed = true;
    continue;
  }

  console.log(`OK: ${dir}/${exe} v${binVersion} (record supported)`);
  verified++;
}

if (verified === 0) {
  console.error('\nNo bundled binaries found. Run prepackage (build + bundle) before packaging.');
  process.exit(1);
}

if (failed) {
  console.error('\nBinary verification failed. Run prepackage (build + bundle) before packaging.');
  process.exit(1);
}

console.log(`\nAll bundled binaries match extension v${expectedVersion}.`);
