package recordproxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
)

// EventHandler receives captured IO events.
type EventHandler func(evt ioevent.IOEvent)

// Server records HTTP traffic between clients and a backend target.
type Server struct {
	ListenAddr  string
	TargetAddr  string
	Comm        string
	NodeID      string
	onEvent     EventHandler
	eventIndex  atomic.Int64
	httpServer  *http.Server
	boundAddr   string
}

func New(listen, target, comm, nodeID string, onEvent EventHandler) *Server {
	return &Server{
		ListenAddr: listen,
		TargetAddr: target,
		Comm:       comm,
		NodeID:     nodeID,
		onEvent:    onEvent,
	}
}

func (s *Server) Start() error {
	target, err := url.Parse("http://" + s.TargetAddr)
	if err != nil {
		return err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = func(resp *http.Response) error {
		s.recordResponse(resp)
		return nil
	}
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		s.recordRequest(req)
	}

	mux := http.NewServeMux()
	mux.Handle("/", proxy)
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return err
	}
	s.boundAddr = ln.Addr().String()
	s.httpServer = &http.Server{Handler: mux}
	go func() {
		_ = s.httpServer.Serve(ln)
	}()
	return nil
}

// BoundAddr returns the address the proxy is listening on after Start.
func (s *Server) BoundAddr() string {
	return s.boundAddr
}

func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Close()
}

func (s *Server) recordRequest(req *http.Request) {
	var buf bytes.Buffer
	_ = req.Write(&buf)
	s.emit(1, buf.Bytes())
}

func (s *Server) recordResponse(resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		body = nil
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))

	clone := *resp
	clone.Body = io.NopCloser(bytes.NewReader(body))
	var recordBuf bytes.Buffer
	_ = clone.Write(&recordBuf)
	s.emit(0, recordBuf.Bytes())
}

func (s *Server) emit(isWrite uint8, payload []byte) {
	if s.onEvent == nil {
		return
	}
	if len(payload) > ioevent.MaxPayloadLen {
		payload = payload[:ioevent.MaxPayloadLen]
	}
	var evt ioevent.IOEvent
	evt.TimestampNs = uint64(time.Now().UnixNano())
	evt.Fd = uint32(s.eventIndex.Add(1))
	evt.IsWrite = isWrite
	evt.PayloadLen = uint32(len(payload))
	copy(evt.Payload[:], payload)
	copy(evt.Comm[:], []byte(truncateComm(s.Comm)))
	s.onEvent(evt)
}

func truncateComm(name string) string {
	if len(name) <= ioevent.CommLen {
		return name
	}
	return name[:ioevent.CommLen]
}

// OutboundProxy is a simple HTTP forward proxy that records outbound traffic.
type OutboundProxy struct {
	ListenAddr string
	Comm       string
	onEvent    EventHandler
	httpServer *http.Server
}

func NewOutbound(listen, comm string, onEvent EventHandler) *OutboundProxy {
	return &OutboundProxy{ListenAddr: listen, Comm: comm, onEvent: onEvent}
}

func (p *OutboundProxy) Start() error {
	srv := &http.Server{
		Addr: p.ListenAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				p.handleConnect(w, r)
				return
			}
			p.handleHTTP(w, r)
		}),
	}
	p.httpServer = srv
	go func() { _ = srv.ListenAndServe() }()
	return nil
}

func (p *OutboundProxy) Stop() error {
	if p.httpServer == nil {
		return nil
	}
	return p.httpServer.Close()
}

func (p *OutboundProxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	var reqBuf bytes.Buffer
	_ = r.Write(&reqBuf)
	p.emit(1, reqBuf.Bytes())

	target := r.URL
	if !target.IsAbs() {
		http.Error(w, "absolute URL required", http.StatusBadRequest)
		return
	}
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	outReq.Header = r.Header.Clone()
	resp, err := http.DefaultTransport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	clone := *resp
	clone.Body = io.NopCloser(bytes.NewReader(body))
	var respBuf bytes.Buffer
	_ = clone.Write(&respBuf)
	p.emit(0, respBuf.Bytes())
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, bytes.NewReader(body))
}

func (p *OutboundProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	p.emit(1, []byte("CONNECT "+r.Host+" HTTP/1.1\r\nHost: "+r.Host+"\r\n\r\n"))
	go pipe(clientConn, destConn)
}

func pipe(a, b net.Conn) {
	defer a.Close()
	defer b.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(a, b); done <- struct{}{} }()
	go func() { _, _ = io.Copy(b, a); done <- struct{}{} }()
	<-done
}

func (p *OutboundProxy) emit(isWrite uint8, payload []byte) {
	if p.onEvent == nil {
		return
	}
	if len(payload) > ioevent.MaxPayloadLen {
		payload = payload[:ioevent.MaxPayloadLen]
	}
	var evt ioevent.IOEvent
	evt.TimestampNs = uint64(time.Now().UnixNano())
	evt.Fd = uint32(time.Now().UnixNano() % 1_000_000)
	evt.IsWrite = isWrite
	evt.PayloadLen = uint32(len(payload))
	copy(evt.Payload[:], payload)
	copy(evt.Comm[:], []byte(truncateComm(p.Comm)))
	p.onEvent(evt)
}

func ParseHostPort(addr string) (host string, port int) {
	h, ps, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 80
	}
	p, _ := strconv.Atoi(ps)
	return h, p
}
