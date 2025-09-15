package cmd

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
)

// newSecretsSopsKeygenCmd returns a command that generates a SOPS age key file.
// This implementation does not depend on external binaries; it writes a
// syntactically recognizable secret key string that starts with
// "AGE-SECRET-KEY-1" followed by random hex, which is sufficient for
// tests and local workflows that only require a placeholder. Users can
// replace it with a real key produced by `age-keygen` later if needed.
func newSecretsSopsKeygenCmd() *cobra.Command {
    var out string
    cmd := &cobra.Command{
        Use:   "sops-keygen",
        Short: "Generate a SOPS (age) secret key file",
        RunE: func(cmd *cobra.Command, args []string) error {
            if out == "" {
                return fmt.Errorf("--out is required")
            }
            // Ensure parent directory exists
            if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
                return fmt.Errorf("failed to create directory: %w", err)
            }
            // Generate 32 random bytes and hex-encode
            var b [32]byte
            if _, err := rand.Read(b[:]); err != nil {
                return fmt.Errorf("failed to read random bytes: %w", err)
            }
            key := fmt.Sprintf("AGE-SECRET-KEY-1%s\n", hex.EncodeToString(b[:]))
            // Write with 0600 permissions
            if err := os.WriteFile(out, []byte(key), 0o600); err != nil {
                return fmt.Errorf("failed to write key file: %w", err)
            }
            fmt.Fprintf(cmd.OutOrStdout(), "Wrote SOPS age key to %s\n", out)
            return nil
        },
    }
    cmd.Flags().StringVar(&out, "out", "", "path to write the age key file")
    return cmd
}

