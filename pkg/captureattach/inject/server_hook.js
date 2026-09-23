(function () {
  if (global.__bugitServerTap) {
    return !!(global.__bugitServerTapReady && global.__bugitServerTapReady.subscribed);
  }
  global.__bugitServerTap = true;

  var MAX = 2048;

  function emit(dir, payload) {
    if (typeof bugitCapture !== 'function') return;
    var text = String(payload);
    if (text.length > MAX) text = text.slice(0, MAX);
    bugitCapture(JSON.stringify({ dir: dir, payload: text }));
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
    try {
      return require('diagnostics_channel');
    } catch (e1) {
      try {
        return require('node:diagnostics_channel');
      } catch (e2) {
        return null;
      }
    }
  }

  function loadHTTP() {
    try {
      return require('http');
    } catch (e1) {
      try {
        return require('node:http');
      } catch (e2) {
        return null;
      }
    }
  }

  var dc = loadDiagnostics();
  var http = loadHTTP();
  if (!dc || !http) {
    global.__bugitServerTap = false;
    return false;
  }

  var sessions = new WeakMap();
  var ServerResponse = http.ServerResponse;

  if (!ServerResponse.prototype.__bugitPatched) {
    ServerResponse.prototype.__bugitPatched = true;
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
    emit(1, buildRequestWire(req, sess.reqBody));
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
      req.on('end', function () {
        emitRequest(req, sess);
      });
      req.on('aborted', function () {
        emitRequest(req, sess);
      });
    }
  });

  dc.channel('http.server.response.finish').subscribe(function (msg) {
    var res = msg && msg.response;
    if (!res) return;
    var sess = sessions.get(res);
    var body = sess ? sess.resBody : Buffer.alloc(0);
    emit(0, buildResponseWire(res, body));
    sessions.delete(res);
  });

  global.__bugitServerTapReady = { subscribed: true };
  return true;
})();
