package main

import (
    "bufio"
    "context"
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"
)

// openCenter-myip
//
// Example Go plugin for openCenter that fetches the public IP address
// from https://icanhazip.com/ over HTTPS and prints it to stdout.
//
// Build:
//   go build -o openCenter-myip .
//
// Install (one of):
//   mkdir -p ~/.config/openCenter/plugins && mv ./openCenter-myip ~/.config/openCenter/plugins/
//   # or set OPENCENTER_PLUGINS_DIR and place the binary there
//   # or place it somewhere on your PATH
//
// Usage (after installation):
//   openCenter myip

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://icanhazip.com/", nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to create request: %v\n", err)
        os.Exit(1)
    }
    req.Header.Set("User-Agent", "openCenter-plugin-myip/1.0 (+https://github.com)")
    req.Header.Set("Accept", "text/plain")

    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
        os.Exit(1)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        fmt.Fprintf(os.Stderr, "unexpected status: %s\n", resp.Status)
        os.Exit(1)
    }

    // Read a single line to avoid trailing newlines or extra content.
    reader := bufio.NewReader(resp.Body)
    line, err := reader.ReadString('\n')
    if err != nil && !strings.Contains(err.Error(), "EOF") {
        // If EOF without newline, we'll still print what we got; only fatal on other errors.
        // For simplicity, treat non-EOF errors as fatal.
        if line == "" { // nothing read
            fmt.Fprintf(os.Stderr, "failed to read response: %v\n", err)
            os.Exit(1)
        }
    }
    fmt.Println(strings.TrimSpace(line))
}

