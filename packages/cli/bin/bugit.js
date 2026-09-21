#!/usr/bin/env node
'use strict';

const { spawnSync } = require('child_process');
const path = require('path');
const fs = require('fs');

function platformPkg() {
  const platform = process.platform;
  const arch = process.arch;
  if (platform === 'win32' && arch === 'x64') return '@bugit/cli-win32-x64';
  if (platform === 'linux' && arch === 'x64') return '@bugit/cli-linux-x64';
  if (platform === 'darwin' && arch === 'arm64') return '@bugit/cli-darwin-arm64';
  if (platform === 'darwin' && arch === 'x64') return '@bugit/cli-darwin-x64';
  return null;
}

function resolveNativeBinary() {
  const pkg = platformPkg();
  if (pkg) {
    try {
      const nativeRoot = path.dirname(require.resolve(pkg + '/package.json'));
      const exe = process.platform === 'win32' ? 'bugit.exe' : 'bugit';
      const candidate = path.join(nativeRoot, 'bin', exe);
      if (fs.existsSync(candidate)) {
        return candidate;
      }
    } catch (_) {
      /* optional dependency not installed */
    }
  }

  const repoBin = path.join(__dirname, '..', '..', 'bin', process.platform === 'win32' ? 'bugit.exe' : 'bugit');
  if (fs.existsSync(repoBin)) {
    return repoBin;
  }

  const sibling = path.join(__dirname, process.platform === 'win32' ? 'bugit.exe' : 'bugit');
  if (fs.existsSync(sibling)) {
    return sibling;
  }

  console.error('BugIT native binary not found. Run `make build` in the BugIT repo or install platform package.');
  process.exit(1);
}

const bin = resolveNativeBinary();
const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 0);
