package gitea

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"syscall"
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

	if err := service.writeCertificates([]string{"10.89.0.11", "192.168.1.100"}, nil); err != nil {
		t.Fatalf("writeCertificates() error = %v", err)
	}

	data, err := os.ReadFile(service.layout.ServerCertPath)
	if err != nil {
		t.Fatalf("read server cert: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
		return
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

func TestWriteCertificatesReplacesForeignOwnedFile(t *testing.T) {
	service, err := NewService(localdev.NewExecutor(), t.TempDir(), DefaultSettings("podman"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	// Simulate a foreign-owned cert file that the current user cannot
	// overwrite: create a real file, then remove write permission on the
	// file while keeping the directory writable. os.WriteFile truncates in
	// place, so a non-writable file must fail the naive path.
	if err := os.WriteFile(service.layout.CACertPath, []byte("stale"), 0o444); err != nil {
		t.Fatalf("seed ca.pem: %v", err)
	}

	if err := service.writeCertificates(nil, nil); err != nil {
		t.Fatalf("writeCertificates() error = %v (expected it to replace the read-only file)", err)
	}

	// The file must now hold a fresh, parseable certificate (not "stale").
	data, err := os.ReadFile(service.layout.CACertPath)
	if err != nil {
		t.Fatalf("read ca.pem: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("ca.pem is not a certificate: %q", string(data[:min(40, len(data))]))
	}
}

func TestWriteCertificatesIncludesURLHostSAN(t *testing.T) {
	service, err := NewService(localdev.NewExecutor(), t.TempDir(), DefaultSettings("podman"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if err := service.writeCertificates([]string{"10.89.0.11"}, []string{"gitea.oc-baremetal", "not-an-ip-but-a-name"}); err != nil {
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
	found := false
	for _, name := range cert.DNSNames {
		if name == "gitea.oc-baremetal" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected gitea.oc-baremetal in DNS SANs, got %v", cert.DNSNames)
	}
}

// TestWriteCertificatesKeyReadableByContainer verifies the container-UID
// re-ownership fallback: when the CLI cannot chown (unprivileged test user),
// key.pem must still be readable by a process running as the container UID.
// Without this the Gitea HTTPS listener fails with "open key.pem: permission
// denied" and waitForAPI times out during gitea-attach-kind.
func TestWriteCertificatesKeyReadableByContainer(t *testing.T) {
	service, err := NewService(localdev.NewExecutor(), t.TempDir(), DefaultSettings("podman"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.layout.Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if err := service.writeCertificates(nil, nil); err != nil {
		t.Fatalf("writeCertificates() error = %v", err)
	}

	info, err := os.Stat(service.layout.ServerKeyPath)
	if err != nil {
		t.Fatalf("stat key.pem: %v", err)
	}
	mode := info.Mode().Perm()
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("key.pem stat did not expose Unix ownership")
	}
	// The container must be able to read the key. Either the key is owned by
	// the container UID with owner-read set, or the unprivileged fallback made
	// it group/world readable. The first case is common on CI runners whose host
	// UID is already the same as the container UID.
	containerOwnedAndReadable := stat.Uid == uint32(giteaContainerUID) && mode&0o400 != 0
	fallbackReadable := mode&0o044 != 0
	if !containerOwnedAndReadable && !fallbackReadable {
		t.Fatalf("key.pem uid=%d mode=%v is not readable by container uid %d", stat.Uid, mode, giteaContainerUID)
	}
	// And the CLI user (owner, when not chowned) must retain read for the
	// next write cycle — owner-read must be set.
	if mode&0o400 == 0 {
		t.Fatalf("key.pem mode %v lost owner read; the CLI cannot rewrite it", mode)
	}
}
