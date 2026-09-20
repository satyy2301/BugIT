import * as http from 'http';
import * as https from 'https';
import * as fs from 'fs';

export interface SnapshotInfo {
  id: string;
  captured_at?: string;
  event_count?: number;
  download_url?: string;
  path?: string;
}

interface ListResponse {
  snapshots?: SnapshotInfo[];
}

function fetchJSON(url: string): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const lib = url.startsWith('https') ? https : http;
    lib
      .get(url, (res) => {
        let data = '';
        res.on('data', (chunk) => (data += chunk.toString()));
        res.on('end', () => {
          if (res.statusCode && res.statusCode >= 400) {
            reject(new Error(`HTTP ${res.statusCode}`));
            return;
          }
          try {
            resolve(JSON.parse(data));
          } catch (err) {
            reject(err);
          }
        });
      })
      .on('error', reject);
  });
}

function downloadToFile(url: string, dest: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const lib = url.startsWith('https') ? https : http;
    lib
      .get(url, (res) => {
        if (res.statusCode && res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          downloadToFile(res.headers.location, dest).then(resolve).catch(reject);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`download HTTP ${res.statusCode}`));
          return;
        }
        const chunks: Buffer[] = [];
        res.on('data', (c) => chunks.push(c));
        res.on('end', () => {
          fs.writeFile(dest, Buffer.concat(chunks), (err) => (err ? reject(err) : resolve()));
        });
      })
      .on('error', reject);
  });
}

export function collectorBase(url: string): string {
  return url.replace(/\/+$/, '');
}

export async function listSnapshots(collectorUrl: string): Promise<SnapshotInfo[]> {
  const parsed = (await fetchJSON(`${collectorBase(collectorUrl)}/v1/snapshots`)) as ListResponse;
  return parsed.snapshots ?? [];
}

export function pickLatest(snapshots: SnapshotInfo[]): SnapshotInfo | undefined {
  if (!snapshots.length) {
    return undefined;
  }
  return [...snapshots].sort((a, b) => {
    const at = a.captured_at ?? '';
    const bt = b.captured_at ?? '';
    return at.localeCompare(bt);
  })[snapshots.length - 1];
}

export function snapshotDownloadURL(collectorUrl: string, snap: SnapshotInfo): string {
  if (snap.download_url && snap.download_url.startsWith('http')) {
    return snap.download_url;
  }
  return `${collectorBase(collectorUrl)}/v1/snapshots/${snap.id}/download`;
}

export async function downloadSnapshot(collectorUrl: string, snap: SnapshotInfo, dest: string): Promise<void> {
  await downloadToFile(snapshotDownloadURL(collectorUrl, snap), dest);
}
