package gitea

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
)

func TestWriteCertificatesIncludesLocalAndKindSANs(t *testing.T) {
	service, err := NewService(localdev.NewExecutor(), t.TempDir(), DefaultSettings("podman"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	if err := service.writeCertificates([]string{"10.89.0.11", "192.168.1.100"}); err != nil {
		t.Fatalf("writeCertificates() error = %v", err)
	}

	data, err := os.ReadFile(service.layout.ServerCertPath)
	if err != nil {
		t.Fatalf("read server cert: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	if len(cert.DNSNames) == 0 || cert.DNSNames[0] != "localhost" {
		t.Fatalf("unexpected DNS names: %v", cert.DNSNames)
	}

	foundGitea := false
	foundKindIP := false
	foundHostIP := false
	for _, name := range cert.DNSNames {
		if name == "gitea" {
			foundGitea = true
		}
	}
	for _, ip := range cert.IPAddresses {
		if ip.String() == "10.89.0.11" {
			foundKindIP = true
		}
		if ip.String() == "192.168.1.100" {
			foundHostIP = true
		}
	}
	if !foundGitea {
		t.Fatalf("expected gitea SAN in %v", cert.DNSNames)
	}
	if !foundKindIP {
		t.Fatalf("expected kind IP SAN in %v", cert.IPAddresses)
	}
	if !foundHostIP {
		t.Fatalf("expected host IP SAN in %v", cert.IPAddresses)
	}
}
