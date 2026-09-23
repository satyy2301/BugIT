import { strict as assert } from 'assert';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { parseYamlValue, resolveBackendPort } from './portResolver';

function shouldSkipCaptureFallback(output: string): boolean {
  const lower = output.toLowerCase();
  return (
    lower.includes('backend listening') ||
    lower.includes('resolve backend inspector') ||
    lower.includes('eaddrinuse') ||
    lower.includes('restart backend') ||
    lower.includes('inspector on :923') ||
    lower.includes('configured backend inspector') ||
    lower.includes('already listening on')
  );
}

assert.equal(shouldSkipCaptureFallback('resolve backend inspector: no target'), true);
assert.equal(shouldSkipCaptureFallback('EADDRINUSE :::14000'), true);
assert.equal(shouldSkipCaptureFallback('plug-and-play local bug capture'), false);

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'bugit-cm-'));
const backend = path.join(tmp, 'backend');
const web = path.join(tmp, 'web');
fs.mkdirSync(backend);
fs.mkdirSync(web);
fs.writeFileSync(path.join(backend, 'package.json'), '{"scripts":{"dev":"node"}}');
fs.writeFileSync(path.join(web, '.env'), 'API_URL=http://localhost:5000\n');
assert.equal(resolveBackendPort(tmp, backend), 5000);

const bugitDir = path.join(backend, '.bugit');
fs.mkdirSync(bugitDir);
fs.writeFileSync(path.join(bugitDir, 'bugit.yaml'), 'app_port: 4000\ninspect_port: 9230\n');
assert.equal(resolveBackendPort(tmp, backend), 4000);
assert.equal(parseYamlValue('inspect_port: 9230\napp_port: 4000\n', 'inspect_port'), '9230');

console.log('captureManager tests ok');
