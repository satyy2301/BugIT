package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultBackendInspectPort is used when :9229 is occupied by another Node process.
	DefaultBackendInspectPort = 9230
	inspectBootstrapName      = "inspect-bootstrap.cjs"
	preloadHookName           = "preload.cjs"
	runWithHooksName          = "run-with-hooks.cjs"
)

// SyncDevScript writes BugIT hook files and patches package.json dev script when needed.
func SyncDevScript(captureRoot string) (inspectPort int, patched bool, message string, err error) {
	captureRoot, err = filepath.Abs(captureRoot)
	if err != nil {
		return 0, false, "", err
	}
	layout, err := EnsureLayout(captureRoot)
	if err != nil {
		return 0, false, "", err
	}

	inspectPort = chooseInspectPort(captureRoot)
	if err := writeInspectBootstrap(layout.BugitDir, inspectPort); err != nil {
		return inspectPort, false, "", err
	}
	if err := writeRunWithHooks(layout.BugitDir, inspectPort); err != nil {
		return inspectPort, false, "", err
	}
	if err := SyncPreloadHook(captureRoot, ""); err != nil {
		return inspectPort, false, "", err
	}

	pkgPath := filepath.Join(captureRoot, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		if err := persistInspectPort(layout.ConfigPath, inspectPort); err != nil {
			return inspectPort, false, "", err
		}
		return inspectPort, false, "", nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return inspectPort, false, "", err
	}
	dev := pkg.Scripts["dev"]
	if dev == "" {
		dev = pkg.Scripts["start"]
	}
	if dev == "" {
		if err := persistInspectPort(layout.ConfigPath, inspectPort); err != nil {
			return inspectPort, false, "", err
		}
		return inspectPort, false, "", nil
	}
	if strings.Contains(dev, ".bugit/"+strings.TrimSuffix(inspectBootstrapName, ".cjs")) ||
		strings.Contains(dev, ".bugit/inspect-bootstrap") ||
		strings.Contains(dev, ".bugit/run-with-hooks.cjs") {
		if err := persistInspectPort(layout.ConfigPath, inspectPort); err != nil {
			return inspectPort, false, "", err
		}
		return inspectPort, false, "", nil
	}

	newDev, changed := patchDevScript(dev, inspectPort)
	if !changed {
		if err := persistInspectPort(layout.ConfigPath, inspectPort); err != nil {
			return inspectPort, false, "", err
		}
		return inspectPort, false, "", nil
	}
	scriptKey := "dev"
	if pkg.Scripts["dev"] == "" {
		scriptKey = "start"
	}
	pkg.Scripts[scriptKey] = newDev
	out, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return inspectPort, false, "", err
	}
	out = append(out, '\n')
	if err := os.WriteFile(pkgPath, out, 0o644); err != nil {
		return inspectPort, false, "", err
	}
	if err := persistInspectPort(layout.ConfigPath, inspectPort); err != nil {
		return inspectPort, false, "", err
	}
	msg := fmt.Sprintf("BugIT configured backend inspector on :%d — restart backend once (Ctrl+C then npm run dev, or nodemon rs)", inspectPort)
	return inspectPort, true, msg, nil
}

func persistInspectPort(configPath string, inspectPort int) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	cfg.InspectPort = inspectPort
	return SaveConfig(configPath, cfg)
}

// SyncPreloadHook writes the direct HTTP ingest preload hook under .bugit/.
func SyncPreloadHook(captureRoot, collectorHTTP string) error {
	captureRoot, err := filepath.Abs(captureRoot)
	if err != nil {
		return err
	}
	layout, err := EnsureLayout(captureRoot)
	if err != nil {
		return err
	}
	if collectorHTTP == "" {
		cfg, _ := LoadConfig(layout.ConfigPath)
		collectorHTTP = cfg.CollectorHTTP
	}
	if collectorHTTP == "" {
		collectorHTTP = DefaultConfig().CollectorHTTP
	}
	content := renderPreloadHook(collectorHTTP)
	path := filepath.Join(layout.BugitDir, preloadHookName)
	return os.WriteFile(path, []byte(content), 0o644)
}

// IsDevScriptPatched reports whether package.json already references BugIT inspect bootstrap.
func IsDevScriptPatched(captureRoot string) bool {
	data, err := os.ReadFile(filepath.Join(captureRoot, "package.json"))
	if err != nil {
		return false
	}
	s := string(data)
	return strings.Contains(s, ".bugit/inspect-bootstrap") || strings.Contains(s, ".bugit/run-with-hooks.cjs")
}

// IsPreloadConfigured reports whether the preload hook file exists.
func IsPreloadConfigured(captureRoot string) bool {
	_, err := os.Stat(filepath.Join(captureRoot, DirName, preloadHookName))
	return err == nil
}

// RecommendedRestartCommand returns a one-line command to restart the backend dev server.
func RecommendedRestartCommand(captureRoot string) string {
	cmd := DetectDevCommand(captureRoot)
	if strings.HasPrefix(cmd, "npm ") {
		return cmd
	}
	return "npm run dev"
}

func chooseInspectPort(captureRoot string) int {
	cfgPath := filepath.Join(captureRoot, DirName, ConfigName)
	if cfg, err := LoadConfig(cfgPath); err == nil && cfg.InspectPort > 0 && cfg.InspectPort != 9229 {
		return cfg.InspectPort
	}
	if inspectPortListening(9229) {
		return findFreeInspectPort()
	}
	return 9229
}

func inspectPortListening(port int) bool {
	client := &http.Client{Timeout: 400 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func findFreeInspectPort() int {
	for port := DefaultBackendInspectPort; port <= 9239; port++ {
		if !inspectPortListening(port) {
			return port
		}
	}
	return DefaultBackendInspectPort
}

func patchDevScript(dev string, inspectPort int) (string, bool) {
	if strings.Contains(dev, ".bugit/inspect-bootstrap") || strings.Contains(dev, ".bugit/run-with-hooks.cjs") {
		return dev, false
	}

	parts := strings.Fields(dev)
	if len(parts) == 0 {
		return dev, false
	}

	inspectFlag := fmt.Sprintf("--inspect=127.0.0.1:%d", inspectPort)
	preload := "-r .bugit/preload.cjs"
	bootstrap := "-r .bugit/inspect-bootstrap.cjs"

	if parts[0] == "npm" {
		return "node .bugit/run-with-hooks.cjs " + dev, true
	}
	if parts[0] == "nodemon" || parts[0] == "node" {
		newParts := append([]string{}, parts[0], bootstrap, preload, inspectFlag)
		newParts = append(newParts, parts[1:]...)
		return strings.Join(newParts, " "), true
	}
	return dev, false
}

func writeInspectBootstrap(bugitDir string, inspectPort int) error {
	content := fmt.Sprintf(`'use strict';
var port = parseInt(process.env.BUGIT_INSPECT_PORT || '%d', 10);
try {
  require('inspector').open(port, '127.0.0.1', false);
} catch (e) {
  if (e && e.code !== 'ERR_INSPECTOR_ALREADY_CONNECTED') {
    console.error('[BugIT] inspect bootstrap failed:', e.message);
  }
}
`, inspectPort)
	return os.WriteFile(filepath.Join(bugitDir, inspectBootstrapName), []byte(content), 0o644)
}

func writeRunWithHooks(bugitDir string, inspectPort int) error {
	content := fmt.Sprintf(`'use strict';
var port = parseInt(process.env.BUGIT_INSPECT_PORT || '%d', 10);
var inspectFlag = '--inspect=127.0.0.1:' + port;
var hookFlags = ['-r', '.bugit/inspect-bootstrap.cjs', '-r', '.bugit/preload.cjs', inspectFlag];
var existing = (process.env.NODE_OPTIONS || '').trim();
var merged = existing ? existing + ' ' + hookFlags.join(' ') : hookFlags.join(' ');
process.env.NODE_OPTIONS = merged;
var args = process.argv.slice(2);
if (args.length === 0) {
  console.error('[BugIT] run-with-hooks: no command provided');
  process.exit(1);
}
var child = require('child_process').spawn(args[0], args.slice(1), {
  stdio: 'inherit',
  env: process.env,
  shell: process.platform === 'win32'
});
child.on('exit', function (code, signal) {
  if (signal) process.kill(process.pid, signal);
  process.exit(code == null ? 1 : code);
});
`, inspectPort)
	return os.WriteFile(filepath.Join(bugitDir, runWithHooksName), []byte(content), 0o644)
}

func renderPreloadHook(collectorHTTP string) string {
	collectorHTTP = strings.TrimRight(collectorHTTP, "/")
	return fmt.Sprintf(`(function () {
  if (global.__bugitPreloadDisabled) return;
  if (global.__bugitPreloadTap) return;
  global.__bugitPreloadTap = true;

  var COLLECTOR = process.env.BUGIT_COLLECTOR_HTTP || %q;
  var COMM = (process.env.BUGIT_COMM || 'node').slice(0, 16);
  var MAX = 2048;
  var nextFD = 1;

  function postEvent(isWrite, payload) {
    var text = String(payload);
    if (text.length > MAX) text = text.slice(0, MAX);
    var body = JSON.stringify({
      node_id: 'local-dev',
      events: [{
        timestamp_ns: Date.now() * 1000000,
        fd: nextFD++,
        is_write: isWrite ? 1 : 0,
        comm: COMM,
        payload: text
      }]
    });
    try {
      var http = require('http');
      var url = new URL(COLLECTOR + '/v1/events');
      var req = http.request({
        hostname: url.hostname,
        port: url.port || 80,
        path: url.pathname,
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(body) }
      });
      req.on('error', function () {});
      req.write(body);
      req.end();
    } catch (e) {}
  }

  function headerLines(headers) {
    var out = '';
    if (!headers) return out;
    for (var k in headers) {
      if (!Object.prototype.hasOwnProperty.call(headers, k)) continue;
      var v = headers[k];
      if (Array.isArray(v)) {
        for (var i = 0; i < v.length; i++) out += k + ': ' + v[i] + '\r\n';
      } else {
        out += k + ': ' + v + '\r\n';
      }
    }
    return out;
  }

  function loadDiagnostics() {
    try { return require('diagnostics_channel'); } catch (e1) {
      try { return require('node:diagnostics_channel'); } catch (e2) { return null; }
    }
  }

  function loadHTTP() {
    try { return require('http'); } catch (e1) {
      try { return require('node:http'); } catch (e2) { return null; }
    }
  }

  var dc = loadDiagnostics();
  var http = loadHTTP();
  if (!dc || !http) return;

  var sessions = new WeakMap();
  var ServerResponse = http.ServerResponse;

  if (!ServerResponse.prototype.__bugitPreloadPatched) {
    ServerResponse.prototype.__bugitPreloadPatched = true;
    var origWrite = ServerResponse.prototype.write;
    var origEnd = ServerResponse.prototype.end;
    ServerResponse.prototype.write = function (chunk, encoding, cb) {
      var sess = sessions.get(this);
      if (sess && chunk != null) {
        var buf = Buffer.isBuffer(chunk) ? chunk : Buffer.from(String(chunk));
        sess.resBody = Buffer.concat([sess.resBody || Buffer.alloc(0), buf]).slice(0, MAX);
      }
      return origWrite.apply(this, arguments);
    };
    ServerResponse.prototype.end = function (chunk, encoding, cb) {
      var sess = sessions.get(this);
      if (sess && chunk != null) {
        var buf = Buffer.isBuffer(chunk) ? chunk : Buffer.from(String(chunk));
        sess.resBody = Buffer.concat([sess.resBody || Buffer.alloc(0), buf]).slice(0, MAX);
      }
      return origEnd.apply(this, arguments);
    };
  }

  function buildRequestWire(req, bodyBuf) {
    var host = (req.headers && req.headers.host) || 'localhost';
    var wire = req.method + ' ' + req.url + ' HTTP/1.1\r\nHost: ' + host + '\r\n';
    wire += headerLines(req.headers);
    wire += '\r\n';
    if (bodyBuf && bodyBuf.length) {
      var room = MAX - wire.length;
      if (room > 0) wire += bodyBuf.toString('utf8', 0, Math.min(bodyBuf.length, room));
    }
    return wire;
  }

  function buildResponseWire(res, bodyBuf) {
    var status = res.statusCode || 0;
    var statusText = res.statusMessage || '';
    var wire = 'HTTP/1.1 ' + status + ' ' + statusText + '\r\n';
    var headers = typeof res.getHeaders === 'function' ? res.getHeaders() : {};
    wire += headerLines(headers);
    wire += '\r\n';
    if (bodyBuf && bodyBuf.length) {
      var room = MAX - wire.length;
      if (room > 0) wire += bodyBuf.toString('utf8', 0, Math.min(bodyBuf.length, room));
    }
    return wire;
  }

  function emitRequest(req, sess) {
    if (sess.emittedRequest) return;
    sess.emittedRequest = true;
    postEvent(1, buildRequestWire(req, sess.reqBody));
  }

  dc.channel('http.server.request.start').subscribe(function (msg) {
    var req = msg && msg.request;
    var res = msg && msg.response;
    if (!req || !res) return;
    var sess = { reqBody: Buffer.alloc(0), resBody: Buffer.alloc(0), emittedRequest: false };
    sessions.set(res, sess);
    if (req.readableEnded || req.complete) {
      emitRequest(req, sess);
    } else {
      req.on('data', function (chunk) {
        sess.reqBody = Buffer.concat([sess.reqBody, chunk]).slice(0, MAX);
      });
      req.on('end', function () { emitRequest(req, sess); });
      req.on('aborted', function () { emitRequest(req, sess); });
    }
  });

  dc.channel('http.server.response.finish').subscribe(function (msg) {
    var res = msg && msg.response;
    if (!res) return;
    var sess = sessions.get(res);
    var body = sess ? sess.resBody : Buffer.alloc(0);
    postEvent(0, buildResponseWire(res, body));
    sessions.delete(res);
  });
})();
`, collectorHTTP)
}
