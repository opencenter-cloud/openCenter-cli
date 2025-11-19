package generator

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/talos"
	"golang.org/x/crypto/curve25519"
)

// GenerateWireGuardConfig creates VPN configuration with server keypair,
// server address/port, and peer configurations.
func (g *generator) GenerateWireGuardConfig(ctx context.Context) (*talos.WireGuardConfig, error) {
	// Get Talos configuration
	talosConfig := talos.DefaultTalosConfig()

	// Generate server keypair
	serverPrivateKey, serverPublicKey, err := generateWireGuardKeypair()
	if err != nil {
		return nil, talos.NewSecurityError(
			"KEYGEN_ERROR",
			"failed to generate WireGuard server keypair",
			nil,
			err,
		)
	}

	// Generate peer configurations
	peers, err := g.generateWireGuardPeers(ctx, 1) // Generate 1 peer by default
	if err != nil {
		return nil, fmt.Errorf("failed to generate WireGuard peers: %w", err)
	}

	config := &talos.WireGuardConfig{
		ServerPublicKey:  serverPublicKey,
		ServerPrivateKey: serverPrivateKey,
		ServerAddress:    "10.0.1.1", // Management network gateway
		ServerPort:       talosConfig.NetworkConfig.WireGuardPort,
		Peers:            peers,
	}

	return config, nil
}

// generateWireGuardKeypair generates a WireGuard private/public keypair.
func generateWireGuardKeypair() (privateKey, publicKey string, err error) {
	// Generate 32 random bytes for private key
	var privateKeyBytes [32]byte
	if _, err := rand.Read(privateKeyBytes[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Clamp the private key (WireGuard requirement)
	privateKeyBytes[0] &= 248
	privateKeyBytes[31] &= 127
	privateKeyBytes[31] |= 64

	// Derive public key from private key using Curve25519
	var publicKeyBytes [32]byte
	curve25519.ScalarBaseMult(&publicKeyBytes, &privateKeyBytes)

	// Encode keys to base64
	privateKey = base64.StdEncoding.EncodeToString(privateKeyBytes[:])
	publicKey = base64.StdEncoding.EncodeToString(publicKeyBytes[:])

	return privateKey, publicKey, nil
}

// generateWireGuardPeers generates peer configurations.
func (g *generator) generateWireGuardPeers(ctx context.Context, count int) ([]talos.WireGuardPeer, error) {
	peers := make([]talos.WireGuardPeer, count)

	for i := 0; i < count; i++ {
		// Generate peer keypair
		_, peerPublicKey, err := generateWireGuardKeypair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate peer %d keypair: %w", i, err)
		}

		// Generate preshared key for additional security
		presharedKey, err := generatePresharedKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate preshared key for peer %d: %w", i, err)
		}

		peers[i] = talos.WireGuardPeer{
			PublicKey: peerPublicKey,
			AllowedIPs: []string{
				"10.0.0.0/8",    // Allow access to all cluster networks
				"172.16.0.0/12", // Private network range
			},
			PresharedKey: presharedKey,
		}
	}

	return peers, nil
}

// generatePresharedKey generates a WireGuard preshared key.
func generatePresharedKey() (string, error) {
	// Generate 32 random bytes for preshared key
	var keyBytes [32]byte
	if _, err := rand.Read(keyBytes[:]); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64
	return base64.StdEncoding.EncodeToString(keyBytes[:]), nil
}
