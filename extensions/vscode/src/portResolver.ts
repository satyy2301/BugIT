import * as fs from 'fs';
import * as path from 'path';

function portFromEnvFile(envPath: string): number | undefined {
  if (!fs.existsSync(envPath)) {
    return undefined;
  }
  const m = fs.readFileSync(envPath, 'utf8').match(/^PORT=(\d+)/m);
  return m ? parseInt(m[1], 10) : undefined;
}

function apiURLPortFromEnv(envPath: string): number | undefined {
  if (!fs.existsSync(envPath)) {
    return undefined;
  }
  const m = fs.readFileSync(envPath, 'utf8').match(
    /(?:NEXT_PUBLIC_API_URL|API_URL|VITE_API_URL)\s*=\s*https?:\/\/[^:]+:(\d+)/im
  );
  return m ? parseInt(m[1], 10) : undefined;
}

export function resolveBackendPort(workspace: string, captureRoot: string): number {
  const yamlPath = path.join(captureRoot, '.bugit', 'bugit.yaml');
  if (fs.existsSync(yamlPath)) {
    const appPortMatch = fs.readFileSync(yamlPath, 'utf8').match(/^app_port:\s*(\d+)/m);
    if (appPortMatch) {
      return parseInt(appPortMatch[1], 10);
    }
  }

  for (const name of ['.env', '.env.local', '.env.development']) {
    const p = portFromEnvFile(path.join(captureRoot, name));
    if (p) {
      return p;
    }
  }

  for (const rel of ['web', 'frontend', 'client', 'apps/web']) {
    const dir = path.join(workspace, rel);
    for (const name of ['.env', '.env.local', '.env.development']) {
      const p = apiURLPortFromEnv(path.join(dir, name));
      if (p) {
        return p;
      }
    }
  }

  return fs.existsSync(path.join(captureRoot, 'package.json')) ? 4000 : 3000;
}

export function parseYamlValue(content: string, key: string): string | undefined {
  const m = content.match(new RegExp(`^${key}:\\s*(.+)$`, 'm'));
  return m ? m[1].trim() : undefined;
}
