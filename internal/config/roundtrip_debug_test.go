package config

import (
    "encoding/json"
    "os"
    "reflect"
    "testing"
)

// This test helps diagnose mismatches between NewDefault -> Save -> Load roundtrip.
// It will be skipped by default; set OPENCENTER_DEBUG_ROUNDTRIP=1 to enable.
func TestRoundTripDebug(t *testing.T) {
    if os.Getenv("OPENCENTER_DEBUG_ROUNDTRIP") == "" {
        t.Skip("set OPENCENTER_DEBUG_ROUNDTRIP=1 to enable")
    }
    dir := t.TempDir()
    t.Setenv("OPENCENTER_CONFIG_DIR", dir)

    orig := NewDefault("test")
    if err := Save(orig); err != nil {
        t.Fatalf("save: %v", err)
    }
    loaded, err := Load("test")
    if err != nil {
        t.Fatalf("load: %v", err)
    }
    if reflect.DeepEqual(orig, loaded) {
        t.Log("roundtrip equal")
        return
    }
    oj, _ := json.MarshalIndent(orig, "", "  ")
    lj, _ := json.MarshalIndent(loaded, "", "  ")
    t.Logf("orig=%s", string(oj))
    t.Logf("loaded=%s", string(lj))
    t.Fatalf("roundtrip mismatch")
}

