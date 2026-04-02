package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type terminalInput interface {
	io.Reader
	Fd() uintptr
}

func readSecretsLoginPassword(cmd *cobra.Command, passwordIn bool) (string, error) {
	stdin := cmd.InOrStdin()
	stdinFile, ok := stdin.(terminalInput)
	interactive := ok && term.IsTerminal(int(stdinFile.Fd()))

	var fd uintptr
	if ok {
		fd = stdinFile.Fd()
	}

	return resolveSecretsLoginPassword(
		stdin,
		passwordIn,
		interactive,
		fd,
		cmd.ErrOrStderr(),
		func(fileDescriptor int) ([]byte, error) {
			return term.ReadPassword(fileDescriptor)
		},
	)
}

func resolveSecretsLoginPassword(
	stdin io.Reader,
	passwordIn bool,
	interactive bool,
	stdinFD uintptr,
	promptWriter io.Writer,
	readPassword func(int) ([]byte, error),
) (string, error) {
	if passwordIn {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("could not read password from stdin: %w", err)
		}

		password := strings.TrimSpace(string(data))
		if password == "" {
			return "", fmt.Errorf("password from stdin cannot be empty")
		}
		return password, nil
	}

	if !interactive {
		return "", fmt.Errorf("refusing to read password from non-interactive stdin; use --password-stdin")
	}

	if _, err := fmt.Fprint(promptWriter, "OpenStack password: "); err != nil {
		return "", fmt.Errorf("write password prompt: %w", err)
	}

	passwordBytes, err := readPassword(int(stdinFD))
	if _, newlineErr := fmt.Fprintln(promptWriter); newlineErr != nil && err == nil {
		err = newlineErr
	}
	if err != nil {
		return "", fmt.Errorf("could not read password from terminal: %w", err)
	}

	password := strings.TrimSpace(string(passwordBytes))
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	return password, nil
}
