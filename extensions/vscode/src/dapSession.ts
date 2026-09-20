import {
  Breakpoint,
  DebugSession,
  InitializedEvent,
  Source,
  StackFrame,
  StoppedEvent,
  TerminatedEvent,
  Thread,
} from '@vscode/debugadapter';
import { DebugProtocol } from '@vscode/debugprotocol';
import { debuggerRequest, parseDebugAddr } from './replayClient';

export interface LaunchRequestArguments extends DebugProtocol.LaunchRequestArguments {
  debugAddr?: string;
}

export class DreReplayDebugSession extends DebugSession {
  private debugAddr = '127.0.0.1:19090';
  private cursor = 0;
  private breakpoints = new Set<number>();

  protected initializeRequest(
    response: DebugProtocol.InitializeResponse,
    _args: DebugProtocol.InitializeRequestArguments,
  ): void {
    response.body = response.body ?? {};
    response.body.supportsConfigurationDoneRequest = true;
    response.body.supportsSetVariable = false;
    this.sendResponse(response);
    this.sendEvent(new InitializedEvent());
  }

  protected async launchRequest(
    response: DebugProtocol.LaunchResponse,
    args: LaunchRequestArguments,
  ): Promise<void> {
    this.debugAddr = args.debugAddr ?? this.debugAddr;
    const { host, port } = parseDebugAddr(this.debugAddr);
    try {
      const state = await debuggerRequest(host, port, { method: 'GetState' });
      this.cursor = Number(state.index ?? 0);
      this.sendResponse(response);
      this.sendEvent(new StoppedEvent('entry', 1));
    } catch (err) {
      this.sendErrorResponse(response, 2001, `Replay debugger not reachable on ${this.debugAddr}: ${err}`);
    }
  }

  protected async setBreakPointsRequest(
    response: DebugProtocol.SetBreakpointsResponse,
    args: DebugProtocol.SetBreakpointsArguments,
  ): Promise<void> {
    const { host, port } = parseDebugAddr(this.debugAddr);
    await debuggerRequest(host, port, { method: 'ClearBreakpoints' });
    this.breakpoints.clear();

    const breakpoints: Breakpoint[] = [];
    for (const srcBp of args.breakpoints ?? []) {
      const index = Math.max(0, (srcBp.line ?? 1) - 1);
      await debuggerRequest(host, port, { method: 'SetBreakpoint', index, enabled: true });
      this.breakpoints.add(index);
      breakpoints.push(new Breakpoint(true, index + 1));
    }
    response.body = { breakpoints };
    this.sendResponse(response);
  }

  protected async continueRequest(
    response: DebugProtocol.ContinueResponse,
    _args: DebugProtocol.ContinueArguments,
  ): Promise<void> {
    const { host, port } = parseDebugAddr(this.debugAddr);
    const state = await debuggerRequest(host, port, { method: 'RunToEvent' });
    this.cursor = Number(state.index ?? this.cursor);
    this.sendResponse(response);
    const reason = state.stopped_reason === 'breakpoint' ? 'breakpoint' : 'step';
    this.sendEvent(new StoppedEvent(reason, 1));
  }

  protected async nextRequest(
    response: DebugProtocol.NextResponse,
    _args: DebugProtocol.NextArguments,
  ): Promise<void> {
    await this.stepOnce(response);
  }

  protected async stepInRequest(
    response: DebugProtocol.StepInResponse,
    _args: DebugProtocol.StepInArguments,
  ): Promise<void> {
    await this.stepOnce(response);
  }

  private async stepOnce(response: DebugProtocol.Response): Promise<void> {
    const { host, port } = parseDebugAddr(this.debugAddr);
    const state = await debuggerRequest(host, port, { method: 'StepForward' });
    this.cursor = Number(state.index ?? this.cursor);
    this.sendResponse(response);
    const reason = state.stopped_reason === 'breakpoint' ? 'breakpoint' : 'step';
    this.sendEvent(new StoppedEvent(reason, 1));
  }

  protected threadsRequest(response: DebugProtocol.ThreadsResponse): void {
    response.body = { threads: [new Thread(1, 'DRE Replay')] };
    this.sendResponse(response);
  }

  protected async stackTraceRequest(
    response: DebugProtocol.StackTraceResponse,
    _args: DebugProtocol.StackTraceArguments,
  ): Promise<void> {
    const { host, port } = parseDebugAddr(this.debugAddr);
    const state = await debuggerRequest(host, port, { method: 'GetState' });
    const idx = Number(state.index ?? 0);
    const label = `event ${idx + 1}`;
    response.body = {
      stackFrames: [
        new StackFrame(
          1,
          label,
          new Source('dre-timeline', 'dre-timeline'),
          idx + 1,
          1,
        ),
      ],
      totalFrames: 1,
    };
    this.sendResponse(response);
  }

  protected disconnectRequest(response: DebugProtocol.DisconnectResponse): void {
    this.sendResponse(response);
    this.sendEvent(new TerminatedEvent());
  }

  protected terminateRequest(response: DebugProtocol.TerminateResponse): void {
    this.sendResponse(response);
    this.sendEvent(new TerminatedEvent());
  }
}
