'use strict';

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const extRoot = path.join(__dirname, '..');
const repoRoot = path.join(extRoot, '..', '..');
const extBin = path.join(extRoot, 'bin');

function run(cmd, args, opts = {}) {
  const result = spawnSync(cmd, args, { stdio: 'inherit', cwd: opts.cwd || repoRoot, shell: opts.shell ?? false });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`${cmd} ${args.join(' ')} exited ${result.status}`);
  }
}

function goBuild(goos, goarch, outPath, mainPkg) {
  const env = { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: '0' };
  const result = spawnSync('go', ['build', '-o', outPath, mainPkg], {
    cwd: repoRoot,
    env,
    stdio: 'inherit',
  });
  if (result.status !== 0) {
    throw new Error(`go build failed for ${goos}/${goarch} -> ${outPath}`);
  }
}

function verifyQuick() {
  const verify = spawnSync(process.execPath, [path.join(__dirname, 'verify-binaries.js')], {
    cwd: extRoot,
    encoding: 'utf8',
  });
  return verify.status === 0;
}

if (verifyQuick()) {
  console.log('Bundled binaries already present and verified — skipping rebuild.');
  process.exit(0);
}

console.log('Building and bundling BugIT binaries for extension...');

if (process.platform === 'win32') {
  const buildPs1 = path.join(repoRoot, 'scripts', 'build.ps1');
  if (fs.existsSync(buildPs1)) {
    run('powershell', ['-ExecutionPolicy', 'Bypass', '-File', buildPs1], { shell: true });
  } else {
    throw new Error('scripts/build.ps1 not found');
  }
} else {
  const bundleSh = path.join(repoRoot, 'scripts', 'bundle-extension-binaries.sh');
  if (fs.existsSync(bundleSh)) {
    run('bash', [bundleSh]);
  } else {
    fs.mkdirSync(path.join(extBin, 'linux-x64'), { recursive: true });
    fs.mkdirSync(path.join(extBin, 'win32-x64'), { recursive: true });
    fs.mkdirSync(path.join(extBin, 'darwin-arm64'), { recursive: true });
    goBuild('linux', 'amd64', path.join(extBin, 'linux-x64', 'bugit'), './bugit-cli/cmd/bugit');
    goBuild('linux', 'amd64', path.join(extBin, 'linux-x64', 'dre-replay'), './dre-replay-cli/cmd/dre-replay');
    goBuild('windows', 'amd64', path.join(extBin, 'win32-x64', 'bugit.exe'), './bugit-cli/cmd/bugit');
    goBuild('windows', 'amd64', path.join(extBin, 'win32-x64', 'dre-replay.exe'), './dre-replay-cli/cmd/dre-replay');
    goBuild('darwin', 'arm64', path.join(extBin, 'darwin-arm64', 'bugit'), './bugit-cli/cmd/bugit');
    goBuild('darwin', 'arm64', path.join(extBin, 'darwin-arm64', 'dre-replay'), './dre-replay-cli/cmd/dre-replay');
  }
}

const verify = spawnSync(process.execPath, [path.join(__dirname, 'verify-binaries.js')], {
  cwd: extRoot,
  stdio: 'inherit',
});
process.exit(verify.status ?? 1);
