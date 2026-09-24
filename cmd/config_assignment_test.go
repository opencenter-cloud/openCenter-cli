package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

type assignmentRunner interface {
	Run()
}

type assignmentRunnerConfig struct {
	Runner assignmentRunner `yaml:"runner" json:"runner"`
}

type assignmentPointerConfig struct {
	Name    *string                   `yaml:"name" json:"name"`
	Options *map[string]string        `yaml:"options" json:"options"`
	Pools   *[]services.IPAddressPool `yaml:"pools" json:"pools"`
}

func TestConfigAssignmentSupportsCommaDelimitedStringSlice(t *testing.T) {
	type config struct {
		Servers []string `yaml:"ntp_servers" json:"ntp_servers"`
	}
	cfg := &config{Servers: []string{"old"}}

	if err := applyConfigAssignments(cfg, []configAssignment{{Path: "ntp_servers", Value: "time-a.example,time-b.example"}}); err != nil {
		t.Fatalf("assign ntp servers: %v", err)
	}
	if want := []string{"time-a.example", "time-b.example"}; !reflect.DeepEqual(cfg.Servers, want) {
		t.Fatalf("ntp servers = %#v, want %#v", cfg.Servers, want)
	}
}

func TestConfigAssignmentSupportsJSONSliceOfStructs(t *testing.T) {
	cfg := &services.MetalLBConfig{}
	value := `[{"name":"public","addresses":["192.0.2.10-192.0.2.20"],"default":true}]`
	if err := processParams([]string{"ip_address_pools=" + value}, cfg); err != nil {
		t.Fatalf("assign MetalLB pools: %v", err)
	}
	if len(cfg.IPAddressPools) != 1 || cfg.IPAddressPools[0].Name != "public" || !cfg.IPAddressPools[0].Default {
		t.Fatalf("unexpected MetalLB pools: %#v", cfg.IPAddressPools)
	}
}

func TestConfigAssignmentSupportsJSONMap(t *testing.T) {
	type config struct {
		Labels map[string]string `yaml:"labels" json:"labels"`
	}
	cfg := &config{}
	if err := applyConfigAssignments(cfg, []configAssignment{{Path: "labels", Value: `{"tier":"edge"}`}}); err != nil {
		t.Fatalf("assign JSON map: %v", err)
	}
	if cfg.Labels["tier"] != "edge" {
		t.Fatalf("labels = %#v, want tier=edge", cfg.Labels)
	}
}

func TestConfigAssignmentSupportsTypedPointers(t *testing.T) {
	cfg := &assignmentPointerConfig{}
	if err := applyConfigAssignments(cfg, []configAssignment{
		{Path: "name", Value: "edge"},
		{Path: "options", Value: `{"mode":"fast"}`},
		{Path: "pools", Value: `[ {"name":"public","addresses":["192.0.2.10"]} ]`},
	}); err != nil {
		t.Fatalf("assign pointer fields: %v", err)
	}
	if cfg.Name == nil || *cfg.Name != "edge" || cfg.Options == nil || (*cfg.Options)["mode"] != "fast" || cfg.Pools == nil || len(*cfg.Pools) != 1 {
		t.Fatalf("unexpected pointer assignments: %#v", cfg)
	}
	if err := applyConfigAssignments(cfg, []configAssignment{{Path: "name", Value: "null"}}); err != nil {
		t.Fatalf("clear pointer field: %v", err)
	}
	if cfg.Name != nil {
		t.Fatalf("name pointer = %q, want nil", *cfg.Name)
	}
}

func TestConfigAssignmentAllowsNumericMapKeys(t *testing.T) {
	cfg := map[string]string{"1": "old"}
	if err := applyConfigAssignments(cfg, []configAssignment{{Path: "1", Value: "new"}}); err != nil {
		t.Fatalf("assign numeric map key: %v", err)
	}
	if cfg["1"] != "new" {
		t.Fatalf("numeric map key = %q, want new", cfg["1"])
	}

	intKeys := map[int]string{1: "old"}
	if err := applyConfigAssignments(intKeys, []configAssignment{{Path: "1", Value: "new"}}); err != nil {
		t.Fatalf("assign integer map key: %v", err)
	}
	if intKeys[1] != "new" {
		t.Fatalf("integer map key = %q, want new", intKeys[1])
	}
}

func TestConfigAssignmentRejectsNonEmptyInterfaceWithoutPanicking(t *testing.T) {
	cfg := &assignmentRunnerConfig{}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("assignment panicked: %v", recovered)
		}
	}()
	err := applyConfigAssignments(cfg, []configAssignment{{Path: "runner", Value: "not-a-runner"}})
	if err == nil || !strings.Contains(err.Error(), "does not implement interface") {
		t.Fatalf("expected useful interface error, got %v", err)
	}
}

func TestConfigAssignmentRejectsInvalidJSONAtomically(t *testing.T) {
	cfg := &services.MetalLBConfig{}
	cfg.IPAddressPools = []services.IPAddressPool{{Name: "existing"}}

	err := processParams([]string{
		`ip_address_pools=[{"name":"new"}]`,
		`l2_advertisements=[not-json]`,
	}, cfg)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
	if len(cfg.IPAddressPools) != 1 || cfg.IPAddressPools[0].Name != "existing" {
		t.Fatalf("invalid assignment changed config: %#v", cfg.IPAddressPools)
	}
}

func TestProcessParamsRejectsUnknownParameter(t *testing.T) {
	cfg := &services.CertManagerConfig{}
	err := processParams([]string{"does_not_exist=value"}, cfg)
	if err == nil || !strings.Contains(err.Error(), "field not found") {
		t.Fatalf("expected unknown parameter error, got %v", err)
	}
}

func TestProcessParamsTraversesEmbeddedServiceFields(t *testing.T) {
	cfg := &services.CertManagerConfig{}
	if err := processParams([]string{"namespace=cert-manager", "source.repo=https://example.test/repo"}, cfg); err != nil {
		t.Fatalf("assign embedded service fields: %v", err)
	}
	if cfg.Namespace != "cert-manager" || cfg.Source.Repo != "https://example.test/repo" {
		t.Fatalf("unexpected embedded fields: namespace=%q source=%#v", cfg.Namespace, cfg.Source)
	}
}
