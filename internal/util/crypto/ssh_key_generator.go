/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crypto

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// SSHKeyPair represents an SSH key pair
type SSHKeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
	KeyType    string
}

// GenerateSSHKey generates a passwordless SSH key pair based on the specified cipher type
// Supported types: ed25519, rsa, ecdsa
// The comment parameter is added to the public key in the format: key-data comment
func GenerateSSHKey(cipherType string) (*SSHKeyPair, error) {
	return GenerateSSHKeyWithComment(cipherType, "")
}

// GenerateSSHKeyWithComment generates a passwordless SSH key pair with a custom comment
// Supported types: ed25519, rsa, ecdsa
// The comment parameter is added to the public key in the format: key-data comment
func GenerateSSHKeyWithComment(cipherType, comment string) (*SSHKeyPair, error) {
	switch cipherType {
	case "ed25519":
		return generateED25519Key(comment)
	case "rsa":
		return generateRSAKey(comment)
	case "ecdsa":
		return generateECDSAKey(comment)
	default:
		return nil, fmt.Errorf("unsupported SSH key cipher type: %s (supported: ed25519, rsa, ecdsa)", cipherType)
	}
}

// generateED25519Key generates an Ed25519 SSH key pair
func generateED25519Key(comment string) (*SSHKeyPair, error) {
	// Generate Ed25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	// Convert to SSH format
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Marshal private key to OpenSSH format (passwordless)
	privKeyPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Encode PEM block to bytes
	privKeyBytes := pem.EncodeToMemory(privKeyPEM)

	// Format public key with comment
	pubKeyBytes := formatPublicKeyWithComment(sshPubKey, comment)

	return &SSHKeyPair{
		PrivateKey: privKeyBytes,
		PublicKey:  pubKeyBytes,
		KeyType:    "ed25519",
	}, nil
}

// generateRSAKey generates a 4096-bit RSA SSH key pair
func generateRSAKey(comment string) (*SSHKeyPair, error) {
	// Generate RSA key pair (4096 bits for security)
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Convert to SSH format
	sshPubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Marshal private key to OpenSSH format (passwordless)
	privKeyPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Encode PEM block to bytes
	privKeyBytes := pem.EncodeToMemory(privKeyPEM)

	// Format public key with comment
	pubKeyBytes := formatPublicKeyWithComment(sshPubKey, comment)

	return &SSHKeyPair{
		PrivateKey: privKeyBytes,
		PublicKey:  pubKeyBytes,
		KeyType:    "rsa",
	}, nil
}

// generateECDSAKey generates an ECDSA SSH key pair using P-521 curve
func generateECDSAKey(comment string) (*SSHKeyPair, error) {
	// Generate ECDSA key pair using P-521 curve (highest security)
	privKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ECDSA key: %w", err)
	}

	// Convert to SSH format
	sshPubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Marshal private key to OpenSSH format (passwordless)
	privKeyPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Encode PEM block to bytes
	privKeyBytes := pem.EncodeToMemory(privKeyPEM)

	// Format public key with comment
	pubKeyBytes := formatPublicKeyWithComment(sshPubKey, comment)

	return &SSHKeyPair{
		PrivateKey: privKeyBytes,
		PublicKey:  pubKeyBytes,
		KeyType:    "ecdsa",
	}, nil
}

// formatPublicKeyWithComment formats an SSH public key with an optional comment
func formatPublicKeyWithComment(pubKey ssh.PublicKey, comment string) []byte {
	authorizedKey := ssh.MarshalAuthorizedKey(pubKey)

	// If no comment provided, return as-is
	if comment == "" {
		return authorizedKey
	}

	// SSH authorized key format is: "keytype base64data [comment]\n"
	// MarshalAuthorizedKey already includes a newline, so we need to trim it,
	// add the comment, then add the newline back
	keyWithoutNewline := authorizedKey[:len(authorizedKey)-1]
	return []byte(fmt.Sprintf("%s %s\n", string(keyWithoutNewline), comment))
}
