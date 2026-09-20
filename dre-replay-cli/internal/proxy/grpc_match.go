package proxy

import (
	"strings"
)

// MatchGRPCUnary detects gRPC-over-HTTP/2 style paths: POST /package.Service/Method
func MatchGRPCUnary(request []byte) (service, method string, ok bool) {
	line := firstLine(request)
	if line == "" {
		return "", "", false
	}
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[0] != "POST" {
		return "", "", false
	}
	path := strings.TrimPrefix(parts[1], "/")
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "", "", false
	}
	return path[:idx], path[idx+1:], true
}

func grpcRequestKey(request []byte) string {
	svc, method, ok := MatchGRPCUnary(request)
	if !ok {
		return ""
	}
	return "GRPC " + svc + "/" + method
}
