package grpctls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// Enabled reports whether TLS env vars are set for server or client.
func ServerEnabled() bool {
	return os.Getenv("DRE_GRPC_TLS_CERT_FILE") != "" && os.Getenv("DRE_GRPC_TLS_KEY_FILE") != ""
}

func ClientEnabled() bool {
	return os.Getenv("DRE_GRPC_TLS_CA_FILE") != ""
}

// ServerCredentials loads a TLS key pair for gRPC server listen.
func ServerCredentials() (credentials.TransportCredentials, error) {
	certFile := os.Getenv("DRE_GRPC_TLS_CERT_FILE")
	keyFile := os.Getenv("DRE_GRPC_TLS_KEY_FILE")
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("DRE_GRPC_TLS_CERT_FILE and DRE_GRPC_TLS_KEY_FILE required")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}), nil
}

// ClientCredentials loads CA bundle for mTLS client dial (optional client cert).
func ClientCredentials() (credentials.TransportCredentials, error) {
	caFile := os.Getenv("DRE_GRPC_TLS_CA_FILE")
	if caFile == "" {
		return nil, fmt.Errorf("DRE_GRPC_TLS_CA_FILE required")
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("invalid CA PEM in %s", caFile)
	}
	cfg := &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}
	certFile := os.Getenv("DRE_GRPC_TLS_CERT_FILE")
	keyFile := os.Getenv("DRE_GRPC_TLS_KEY_FILE")
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(cfg), nil
}
