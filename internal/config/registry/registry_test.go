package registry

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

type demoConfig struct {
	Name string `yaml:"name"`
}

type otherConfig struct {
	Count int `yaml:"count"`
}

// resetRegistry clears registrations so tests are isolated from init-time
// registrations in other packages.
func resetRegistry() {
	registryLock.Lock()
	defer registryLock.Unlock()
	serviceRegistry = make(map[string]reflect.Type)
}

func TestRegisterAndGet(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	RegisterServiceConfig("demo", demoConfig{})

	got := GetServiceConfigType("demo")
	if got == nil {
		t.Fatal("expected demo type to be registered")
	}
	if got.Kind() != reflect.Struct {
		t.Fatalf("expected struct kind, got %v", got.Kind())
	}
	if got.NumField() != 1 {
		t.Fatalf("expected 1 field, got %d", got.NumField())
	}
}

func TestGetUnregisteredReturnsNil(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	if got := GetServiceConfigType("missing"); got != nil {
		t.Fatalf("expected nil for unregistered service, got %v", got)
	}
	if IsRegistered("missing") {
		t.Fatal("expected IsRegistered to be false for unregistered service")
	}
}

func TestReRegisterOverwrites(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	RegisterServiceConfig("demo", demoConfig{})
	RegisterServiceConfig("demo", otherConfig{})

	got := GetServiceConfigType("demo")
	if got.NumField() != 1 {
		t.Fatal("expected single field")
	}
	if got.Field(0).Name != "Count" {
		t.Fatalf("expected re-registration to overwrite, got field %q", got.Field(0).Name)
	}
}

func TestRegisteredServicesList(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	if services := GetRegisteredServices(); len(services) != 0 {
		t.Fatalf("expected empty list, got %v", services)
	}

	RegisterServiceConfig("alpha", demoConfig{})
	RegisterServiceConfig("beta", otherConfig{})

	services := GetRegisteredServices()
	sort.Strings(services)
	want := []string{"alpha", "beta"}
	if len(services) != 2 || services[0] != want[0] || services[1] != want[1] {
		t.Fatalf("expected %v, got %v", want, services)
	}
}

func TestPointerRegistrationDistinguishedFromValue(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	RegisterServiceConfig("value", demoConfig{})
	RegisterServiceConfig("pointer", &demoConfig{})

	valueType := GetServiceConfigType("value")
	pointerType := GetServiceConfigType("pointer")

	if valueType == nil || pointerType == nil {
		t.Fatal("expected both registrations")
	}
	if valueType.Kind() != reflect.Struct {
		t.Fatalf("value registration should record struct kind, got %v", valueType.Kind())
	}
	if pointerType.Kind() != reflect.Pointer {
		t.Fatalf("pointer registration should record ptr kind, got %v", pointerType.Kind())
	}
}

func TestConcurrentRegisterAndRead(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("svc-%d", i)
			RegisterServiceConfig(name, demoConfig{})
			_ = GetServiceConfigType(name)
			_ = IsRegistered(name)
			_ = GetRegisteredServices()
		}(i)
	}
	wg.Wait()
}

func TestProperty_RegisterGetRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)
	properties.Property("every registered name resolves to a struct type", prop.ForAll(
		func(names []string) bool {
			if len(names) == 0 {
				return true
			}
			resetRegistry()
			defer resetRegistry()

			for _, name := range names {
				if strings.TrimSpace(name) == "" {
					continue
				}
				RegisterServiceConfig(name, demoConfig{})
			}

			for _, name := range names {
				if strings.TrimSpace(name) == "" {
					continue
				}
				typ := GetServiceConfigType(name)
				if typ == nil || typ.Kind() != reflect.Struct {
					return false
				}
				if !IsRegistered(name) {
					return false
				}
			}

			unique := 0
			seen := make(map[string]bool, len(names))
			for _, name := range names {
				if strings.TrimSpace(name) == "" || seen[name] {
					continue
				}
				seen[name] = true
				unique++
			}
			return len(GetRegisteredServices()) == unique
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}
