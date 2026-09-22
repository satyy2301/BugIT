import * as cp from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';

export interface BugitBinaryInfo {
  path: string;
  version: string;
  supportsRecord: boolean;
  usageText: string;
}

function readExtensionVersion(): string {
  const pkgPath = path.join(__dirname, '..', 'package.json');
  return JSON.parse(fs.readFileSync(pkgPath, 'utf8')).version as string;
}

const EXTENSION_VERSION = readExtensionVersion();

function runSync(bin: string, args: string[]): { stdout: string; stderr: string; status: number | null } {
  try {
    const result = cp.spawnSync(bin, args, { encoding: 'utf8', timeout: 10000, windowsHide: true });
    return {
      stdout: result.stdout ?? '',
      stderr: result.stderr ?? '',
      status: result.status,
    };
  } catch (err) {
    return { stdout: '', stderr: String(err), status: 1 };
  }
}

function parseVersion(output: string): string {
  const match = output.match(/(\d+\.\d+\.\d+)/);
  return match ? match[1] : '';
}

export function inspectBugitBinary(bugitBin: string): BugitBinaryInfo {
  const versionResult = runSync(bugitBin, ['version']);
  const versionOut = `${versionResult.stdout}${versionResult.stderr}`;
  const version = parseVersion(versionOut);

  const recordHelp = runSync(bugitBin, ['record', '--help']);
  const recordText = `${recordHelp.stdout}${recordHelp.stderr}`;
  const supportsRecord = recordHelp.status === 0 && recordText.includes('record');

  let usageText = recordText;
  if (!usageText.trim()) {
    const usageResult = runSync(bugitBin, ['capture', '--help']);
    usageText = `${usageResult.stdout}${usageResult.stderr}`;
  }

  return { path: bugitBin, version, supportsRecord, usageText };
}

export function bundledBugitPath(context: vscode.ExtensionContext): string {
  const plat =
    process.platform === 'win32' ? 'win32-x64' : process.platform === 'darwin' ? 'darwin-arm64' : 'linux-x64';
  const exe = process.platform === 'win32' ? 'bugit.exe' : 'bugit';
  return path.join(context.extensionPath, 'bin', plat, exe);
}

export function extensionVersion(): string {
  return EXTENSION_VERSION;
}

export function validateBugitBinary(info: BugitBinaryInfo): string | undefined {
  if (!fs.existsSync(info.path)) {
    return `BugIT binary not found: ${info.path}`;
  }
  if (!info.version) {
    return `Could not read version from ${info.path}. Run BugIT: Doctor for details.`;
  }
  if (info.version !== EXTENSION_VERSION) {
    return (
      `Bundled CLI is v${info.version} but extension is v${EXTENSION_VERSION}. ` +
      `Reinstall will not fix this until a new release — set bugit.bugitBin to a freshly built binary or build from the BugIT repo.`
    );
  }
  if (!info.supportsRecord) {
    return (
      `BugIT CLI at ${info.path} does not support "bugit record" (found v${info.version}). ` +
      `Set bugit.bugitBin to a v${EXTENSION_VERSION} binary or update the extension when a fixed release is available.`
    );
  }
  return undefined;
}

export async function preflightBugitBinary(bugitBin: string): Promise<BugitBinaryInfo | undefined> {
  const info = inspectBugitBinary(bugitBin);
  const err = validateBugitBinary(info);
  if (err) {
    vscode.window.showErrorMessage(err);
    return undefined;
  }
  return info;
}

export function formatDoctorReport(info: BugitBinaryInfo, workspace?: string): string {
  const lines = [
    'BugIT Doctor',
    `Extension version: ${EXTENSION_VERSION}`,
    `Binary path: ${info.path}`,
    `Binary version: ${info.version || '(unknown)'}`,
    `record subcommand: ${info.supportsRecord ? 'yes' : 'NO — recording will fail'}`,
  ];
  if (info.version && info.version !== EXTENSION_VERSION) {
    lines.push(`WARN: version mismatch (extension ${EXTENSION_VERSION} vs CLI ${info.version})`);
  }
  if (workspace) {
    lines.push(`Workspace: ${workspace}`);
    const doctor = runSync(info.path, ['doctor', '--root', workspace]);
    if (doctor.stdout.trim()) {
      lines.push('', doctor.stdout.trim());
    }
    if (doctor.stderr.trim()) {
      lines.push(doctor.stderr.trim());
    }
  }
  return lines.join('\n');
}
