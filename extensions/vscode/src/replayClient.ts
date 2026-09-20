import * as net from 'net';

export interface DebuggerResponse {
  index?: number;
  total?: number;
  stopped_reason?: string;
  pid?: number;
  tid?: number;
  comm?: string;
  event?: { summary?: string };
}

export function parseDebugAddr(addr: string): { host: string; port: number } {
  const [host, portStr] = addr.includes(':') ? addr.split(':') : ['127.0.0.1', '19090'];
  return { host, port: parseInt(portStr, 10) };
}

export function debuggerRequest(
  host: string,
  port: number,
  body: Record<string, unknown>,
): Promise<DebuggerResponse> {
  return new Promise((resolve, reject) => {
    const sock = net.createConnection({ host, port }, () => {
      sock.write(JSON.stringify(body) + '\n');
    });
    let data = '';
    let settled = false;
    const finish = (err?: Error, result?: DebuggerResponse) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      sock.destroy();
      if (err) {
        reject(err);
      } else {
        resolve(result!);
      }
    };
    sock.on('data', (chunk) => {
      data += chunk.toString();
      try {
        finish(undefined, JSON.parse(data.trim()) as DebuggerResponse);
      } catch {
        // wait for more
      }
    });
    sock.on('error', (err) => finish(err));
    const timer = setTimeout(() => finish(new Error('timeout')), 2000);
  });
}

export async function isDebuggerReachable(debugAddr: string): Promise<boolean> {
  const { host, port } = parseDebugAddr(debugAddr);
  try {
    await debuggerRequest(host, port, { method: 'GetState' });
    return true;
  } catch {
    return false;
  }
}
