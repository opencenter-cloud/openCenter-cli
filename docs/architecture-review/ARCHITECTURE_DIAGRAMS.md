# Architecture Diagrams: Current vs Proposed

**Project**: opencenter-cli  
**Review Date**: February 4, 2026

## Table of Contents

- [Overview](#overview)
- [Current Architecture](#current-architecture)
- [Proposed Architecture](#proposed-architecture)
- [Migration Path](#migration-path)
- [Component Comparisons](#component-comparisons)

## Overview

This document provides visual representations of the current and proposed architectures for opencenter-cli. The diagrams use Mermaid syntax for clarity and can be rendered in most modern markdown viewers.

### Legend

```
┌─────────┐
│ Package │  = Go package
└─────────┘

┌─────────┐
│ Service │  = Service implementation
└─────────┘

───────────  = Dependency/Import
═══════════  = Strong coupling
- - - - - -  = Weak coupling/Interface
```

## Current Architecture

### High-Level System Architecture


```mermaid
graph TB
    subgraph "CLI Layer"
        CMD[cmd/]
        ROOT[root.go]
        CLUSTER[cluster_*.go]
    end
    
    subgraph "Dependency Injection"
        DI[internal/di/]
        CONTAINER[container.go]
        PROVIDERS[providers.go]
    end
    
    subgraph "Core Services - DUPLICATE SYSTEM 1"
        CONFIG_SVC[internal/config/services/]
        SVC_INTERFACES[interfaces.go]
        SVC_VALIDATORS[dependency_validator.go]
        SVC_IMPLS[11+ service implementations]
    end
    
    subgraph "Core Services - DUPLICATE SYSTEM 2"
        SERVICES[internal/services/]
        REGISTRY[registry.go]
        PLUGIN[plugin.go]
        PLUGINS[plugins/11+ implementations]
    end
    
    subgraph "Configuration"
        CONFIG[internal/config/]
        CONFIG_MGR[manager.go]
        CONFIG_TYPES[types_*.go]
        CONFIG_LOADER[loader.go]
    end
    
    subgraph "GitOps Generation"
        GITOPS[internal/gitops/]
        GENERATOR[generator.go]
        PIPELINE[pipeline.go]
        WORKSPACE[workspace.go]
    end
    
    subgraph "Template Engine"
        TEMPLATE[internal/template/]
        ENGINE[engine.go]
        REGISTRY_T[registry.go]
        CACHE[cache.go]
    end
    
    subgraph "Utilities - FRAGMENTED"
        UTIL[internal/util/]
        CRYPTO[crypto/]
        ERRORS[errors/]
        FILES[files/]
        SECURITY[security/]
    end
    
    CMD --> DI
    CMD --> CONFIG
    CMD --> SERVICES
    CMD --> CONFIG_SVC
    
    DI --> CONTAINER
    DI --> PROVIDERS
    PROVIDERS --> CONFIG_MGR
    PROVIDERS --> REGISTRY
    
    CONFIG_SVC -.-> SVC_INTERFACES
    CONFIG_SVC -.-> SVC_VALIDATORS
    CONFIG_SVC -.-> SVC_IMPLS
    
    SERVICES -.-> REGISTRY
    SERVICES -.-> PLUGIN
    SERVICES -.-> PLUGINS
    
    GITOPS --> GENERATOR
    GITOPS --> PIPELINE
    GITOPS --> WORKSPACE
    GITOPS --> TEMPLATE
    
    TEMPLATE --> ENGINE
    TEMPLATE --> REGISTRY_T
    TEMPLATE --> CACHE
    
    CONFIG --> CONFIG_MGR
    CONFIG --> CONFIG_TYPES
    CONFIG --> CONFIG_LOADER
    
    style CONFIG_SVC fill:#ffcccc
    style SERVICES fill:#ffcccc
    style SVC_IMPLS fill:#ffcccc
    style PLUGINS fill:#ffcccc
```

### Problem: Dual Service Architecture


```mermaid
graph LR
    subgraph "System 1: internal/config/services"
        CS1[cert-manager.go]
        CS2[cilium.go]
        CS3[harbor.go]
        CS4[keycloak.go]
        CS5[loki.go]
        CS6[+6 more services]
        CSV[dependency_validator.go]
    end
    
    subgraph "System 2: internal/services/plugins"
        PS1[cert-manager.go]
        PS2[cilium.go]
        PS3[harbor.go]
        PS4[keycloak.go]
        PS5[loki.go]
        PS6[+6 more services]
        PSR[registry.go]
    end
    
    CONFIG[Configuration] --> CS1
    CONFIG --> CS2
    CONFIG --> CS3
    
    GITOPS[GitOps Generation] --> PS1
    GITOPS --> PS2
    GITOPS --> PS3
    
    style CS1 fill:#ffcccc
    style CS2 fill:#ffcccc
    style CS3 fill:#ffcccc
    style PS1 fill:#ffcccc
    style PS2 fill:#ffcccc
    style PS3 fill:#ffcccc
```

**Issues**:
- 11+ services implemented twice
- Different validation logic in each system
- Confusion about which system to use
- Changes must be made in both places
- Inconsistent behavior between systems

### Error Handling Fragmentation

```mermaid
graph TB
    subgraph "Error System 1: internal/config/errors.go"
        CE1[NewFileError]
        CE2[NewValidationError]
        CE3[NewPathError]
        CE4[NewParseError]
        CE5[WrapFileError]
        CE6[IsFileNotFoundError]
    end
    
    subgraph "Error System 2: internal/util/errors/"
        UE1[HandleError]
        UE2[CreateValidationError]
        UE3[NewDefaultErrorHandler]
        UE4[StructuredError]
    end
    
    subgraph "Error System 3: internal/config/flags/errors.go"
        FE1[ConfigError]
        FE2[ErrorBuilder]
        FE3[ErrorReporter]
        FE4[CreateTemplatedError]
    end
    
    CMD[Commands] --> CE1
    CMD --> UE1
    CMD --> FE1
    
    CONFIG[Config Package] --> CE2
    CONFIG --> UE2
    
    SERVICES[Services] --> CE3
    SERVICES --> UE3
    
    style CE1 fill:#ffffcc
    style CE2 fill:#ffffcc
    style UE1 fill:#ffffcc
    style UE2 fill:#ffffcc
    style FE1 fill:#ffffcc
```

**Issues**:
- Three separate error creation systems
- Inconsistent error formats
- Duplicate validation error logic
- No unified error handling strategy

## Proposed Architecture

### Unified System Architecture

```mermaid
graph TB
    subgraph "CLI Layer"
        CMD[cmd/]
        ROOT[root.go]
        CLUSTER[cluster_*.go]
    end
    
    subgraph "Dependency Injection"
        DI[internal/di/]
        CONTAINER[container.go]
        PROVIDERS[providers.go]
    end
    
    subgraph "Unified Service System"
        SERVICES[internal/services/]
        REGISTRY[registry.go]
        DEFINITION[definition.go]
        LIFECYCLE[lifecycle.go]
        PLUGINS[plugins/]
        ADAPTERS[config_adapters/]
    end
    
    subgraph "Configuration"
        CONFIG[internal/config/]
        CONFIG_MGR[manager.go]
        CONFIG_TYPES[types_*.go]
        CONFIG_LOADER[loader.go]
    end
    
    subgraph "Unified Validation"
        VALIDATION[internal/core/validation/]
        ENGINE[engine.go]
        VALIDATORS[validators/]
        RULES[rules.go]
    end
    
    subgraph "GitOps Generation"
        GITOPS[internal/gitops/]
        GENERATOR[generator.go]
        PIPELINE[pipeline.go]
        WORKSPACE[workspace.go]
    end
    
    subgraph "Template Engine"
        TEMPLATE[internal/template/]
        ENGINE_T[engine.go]
        REGISTRY_T[registry.go]
        CACHE[cache.go]
    end
    
    subgraph "Unified Utilities"
        UTIL[internal/util/]
        CRYPTO_U[crypto/unified.go]
        ERRORS_U[errors/structured.go]
        FILES_U[files/operations.go]
    end
    
    CMD --> DI
    CMD --> CONFIG
    CMD --> SERVICES
    
    DI --> CONTAINER
    DI --> PROVIDERS
    PROVIDERS --> CONFIG_MGR
    PROVIDERS --> REGISTRY
    PROVIDERS --> ENGINE
    
    SERVICES --> REGISTRY
    SERVICES --> DEFINITION
    SERVICES --> LIFECYCLE
    SERVICES --> PLUGINS
    SERVICES --> ADAPTERS
    
    SERVICES --> VALIDATION
    CONFIG --> VALIDATION
    GITOPS --> VALIDATION
    
    VALIDATION --> ENGINE
    VALIDATION --> VALIDATORS
    VALIDATION --> RULES
    
    GITOPS --> GENERATOR
    GITOPS --> PIPELINE
    GITOPS --> WORKSPACE
    GITOPS --> TEMPLATE
    
    TEMPLATE --> ENGINE_T
    TEMPLATE --> REGISTRY_T
    TEMPLATE --> CACHE
    
    CONFIG --> CONFIG_MGR
    CONFIG --> CONFIG_TYPES
    CONFIG --> CONFIG_LOADER
    
    style SERVICES fill:#ccffcc
    style VALIDATION fill:#ccffcc
    style UTIL fill:#ccffcc
```

### Unified Service Architecture

```mermaid
graph TB
    subgraph "Service Registry"
        REGISTRY[ServiceRegistry]
        DEFINITION[ServiceDefinition]
        LIFECYCLE[LifecycleHooks]
    end
    
    subgraph "Service Plugins"
        PLUGIN_IF[ServicePlugin Interface]
        CERT[CertManagerPlugin]
        CILIUM[CiliumPlugin]
        HARBOR[HarborPlugin]
        KEYCLOAK[KeycloakPlugin]
        LOKI[LokiPlugin]
        MORE[+6 more plugins]
    end
    
    subgraph "Configuration Adapters"
        ADAPTER[ConfigAdapter]
        CERT_A[CertManagerAdapter]
        CILIUM_A[CiliumAdapter]
        HARBOR_A[HarborAdapter]
    end
    
    subgraph "Validation Engine"
        VALIDATOR[ValidationEngine]
        SVC_VAL[ServiceValidator]
        DEP_VAL[DependencyValidator]
    end
    
    REGISTRY --> DEFINITION
    REGISTRY --> LIFECYCLE
    REGISTRY --> VALIDATOR
    
    DEFINITION --> PLUGIN_IF
    PLUGIN_IF --> CERT
    PLUGIN_IF --> CILIUM
    PLUGIN_IF --> HARBOR
    PLUGIN_IF --> KEYCLOAK
    PLUGIN_IF --> LOKI
    PLUGIN_IF --> MORE
    
    CERT --> CERT_A
    CILIUM --> CILIUM_A
    HARBOR --> HARBOR_A
    
    VALIDATOR --> SVC_VAL
    VALIDATOR --> DEP_VAL
    
    style REGISTRY fill:#ccffcc
    style VALIDATOR fill:#ccffcc
    style ADAPTER fill:#ccffcc
```

**Benefits**:
- Single source of truth for services
- Consistent validation across all services
- Clear separation: plugins for logic, adapters for config
- Unified lifecycle management
- Extensible plugin system

### Unified Error Handling

```mermaid
graph TB
    subgraph "Unified Error System"
        STRUCTURED[StructuredError]
        FACTORY[ErrorFactory]
        HANDLER[ErrorHandler]
        FORMATTER[ErrorFormatter]
    end
    
    subgraph "Error Types"
        FILE[FileError]
        VALIDATION[ValidationError]
        PATH[PathError]
        PARSE[ParseError]
        CONFIG[ConfigError]
        TEMPLATE[TemplateError]
    end
    
    subgraph "Error Context"
        CONTEXT[ErrorContext]
        SUGGESTIONS[Suggestions]
        METADATA[Metadata]
    end
    
    FACTORY --> STRUCTURED
    STRUCTURED --> FILE
    STRUCTURED --> VALIDATION
    STRUCTURED --> PATH
    STRUCTURED --> PARSE
    STRUCTURED --> CONFIG
    STRUCTURED --> TEMPLATE
    
    STRUCTURED --> CONTEXT
    CONTEXT --> SUGGESTIONS
    CONTEXT --> METADATA
    
    HANDLER --> FORMATTER
    FORMATTER --> STRUCTURED
    
    style STRUCTURED fill:#ccffcc
    style FACTORY fill:#ccffcc
    style HANDLER fill:#ccffcc
```

**Benefits**:
- Single error type with all necessary fields
- Consistent error formatting
- Automatic suggestion generation
- Unified error handling strategy
- Easy to extend with new error types

## Migration Path

### Phase 1: Foundation (Week 1)

```mermaid
graph LR
    subgraph "Current State"
        E1[Error System 1]
        E2[Error System 2]
        E3[Error System 3]
    end
    
    subgraph "Migration"
        UNIFIED[Create Unified Error System]
        MIGRATE[Migrate Error Calls]
        TEST[Update Tests]
    end
    
    subgraph "Target State"
        SINGLE[Single Error System]
    end
    
    E1 --> UNIFIED
    E2 --> UNIFIED
    E3 --> UNIFIED
    UNIFIED --> MIGRATE
    MIGRATE --> TEST
    TEST --> SINGLE
    
    style UNIFIED fill:#ffffcc
    style SINGLE fill:#ccffcc
```

### Phase 2: Core Services (Week 2)

```mermaid
graph LR
    subgraph "Current State"
        CS[Config Services]
        PS[Plugin Services]
    end
    
    subgraph "Migration"
        ANALYZE[Analyze Both Systems]
        DESIGN[Design Unified System]
        IMPLEMENT[Implement Registry]
        MIGRATE_S[Migrate Services]
    end
    
    subgraph "Target State"
        UNIFIED_S[Unified Service System]
    end
    
    CS --> ANALYZE
    PS --> ANALYZE
    ANALYZE --> DESIGN
    DESIGN --> IMPLEMENT
    IMPLEMENT --> MIGRATE_S
    MIGRATE_S --> UNIFIED_S
    
    style IMPLEMENT fill:#ffffcc
    style UNIFIED_S fill:#ccffcc
```

### Phase 3: Integration (Week 3)

```mermaid
graph LR
    subgraph "Consolidation"
        CRYPTO[Consolidate Crypto]
        FILES[Unify File I/O]
        CONFIG_OPT[Optimize Config]
    end
    
    subgraph "Integration"
        INTEGRATE[Integrate Systems]
        OPTIMIZE[Optimize Performance]
        VALIDATE[Validate Changes]
    end
    
    subgraph "Target State"
        COMPLETE[Integrated System]
    end
    
    CRYPTO --> INTEGRATE
    FILES --> INTEGRATE
    CONFIG_OPT --> INTEGRATE
    INTEGRATE --> OPTIMIZE
    OPTIMIZE --> VALIDATE
    VALIDATE --> COMPLETE
    
    style INTEGRATE fill:#ffffcc
    style COMPLETE fill:#ccffcc
```

### Phase 4: Cleanup (Week 4)

```mermaid
graph LR
    subgraph "Cleanup"
        REMOVE[Remove Old Code]
        UPDATE_DOCS[Update Documentation]
        TEST_ALL[Comprehensive Testing]
    end
    
    subgraph "Validation"
        BENCHMARK[Performance Benchmarks]
        SECURITY[Security Audit]
        REVIEW[Code Review]
    end
    
    subgraph "Target State"
        PRODUCTION[Production Ready]
    end
    
    REMOVE --> UPDATE_DOCS
    UPDATE_DOCS --> TEST_ALL
    TEST_ALL --> BENCHMARK
    BENCHMARK --> SECURITY
    SECURITY --> REVIEW
    REVIEW --> PRODUCTION
    
    style PRODUCTION fill:#ccffcc
```

## Component Comparisons

### Service System Comparison

| Aspect | Current (Dual System) | Proposed (Unified) |
|--------|----------------------|-------------------|
| **Service Definitions** | 2 locations (11+ duplicates) | 1 location |
| **Validation** | Scattered, inconsistent | Centralized, consistent |
| **Lifecycle** | Partial support | Full lifecycle hooks |
| **Dependencies** | Manual tracking | Automatic resolution |
| **Extensibility** | Limited | Plugin-based |
| **Testing** | Duplicate tests | Single test suite |
| **Maintenance** | High (2x effort) | Low (1x effort) |

### Error Handling Comparison

| Aspect | Current (Triple System) | Proposed (Unified) |
|--------|------------------------|-------------------|
| **Error Types** | 3 separate systems | 1 unified system |
| **Consistency** | Inconsistent formats | Consistent format |
| **Suggestions** | Partial support | Automatic suggestions |
| **Context** | Limited | Rich context |
| **Testing** | Fragmented | Centralized |
| **Maintenance** | High (3x effort) | Low (1x effort) |

### Validation Comparison

| Aspect | Current (Scattered) | Proposed (Unified) |
|--------|-------------------|-------------------|
| **Validators** | Multiple locations | Single engine |
| **Dependencies** | Manual checking | Automatic resolution |
| **Circular Deps** | Partial detection | Full detection |
| **Extensibility** | Limited | Plugin-based |
| **Testing** | Duplicate tests | Single test suite |
| **Maintenance** | Medium effort | Low effort |

## Conclusion

The proposed architecture eliminates duplication, provides clear separation of concerns, and establishes a single source of truth for services, errors, and validation. The migration path is designed to minimize risk through phased implementation with comprehensive testing at each stage.

**Key Improvements**:
- 20-25% code reduction through service unification
- 10-15% code reduction through error handling consolidation
- 10-15% code reduction through validation consolidation
- Improved maintainability and developer experience
- Consistent behavior across all components
- Clear extension points for future features

**Next Steps**: Review these diagrams with the development team and proceed with Phase 1 of the refactoring roadmap.
