# Architectural Diagrams: Current vs. Proposed

## Current Architecture (As-Is)

```mermaid
graph TB
    subgraph "CLI Layer"
        CMD[cmd/root.go<br/>Command Handlers]
        CLUSTER[cmd/cluster_*.go<br/>Cluster Commands]
        CONFIG_CMD[cmd/config_*.go<br/>Config Commands]
        SOPS_CMD[cmd/sops_*.go<br/>SOPS Commands]
    end

    subgraph "Service Layer - FRAGMENTED"
        CLUSTER_SVC[internal/cluster/<br/>Cluster Services]
        
        subgraph "Config Management - 2 SYSTEMS"
            CONFIG_LEGACY[config.go<br/>Legacy Functions]
            CONFIG_MGR[manager.go<br/>ConfigurationManager]
            CONFIG_BUILDER[builder.go<br/>FluentConfigBuilder]
        end
        
        subgraph "Validation - SCATTERED"
            VAL_CONFIG[config/validator.go]
            VAL_SOPS[sops/validator.go]
            VAL_GITOPS[gitops/validators.go]
            VAL_SERVICES[services/plugins/<br/>15+ validators]
        end
        
        GITOPS[internal/gitops/<br/>GitOps Generation]
        SOPS[internal/sops/<br/>Secrets Management]
    end

    subgraph "Infrastructure Layer"
        DI_ROOT[cmd/root.go<br/>DI Init - DUPLICATE]
        DI_SETUP[di/setup.go<br/>DI Init - DUPLICATE]
        DI_CONTAINER[di/container.go<br/>DI Container]
        
        CLOUD[cloud/factory.go<br/>Provider Factory]
        PROVISION[provision/<br/>Provisioning]
    end

    subgraph "Utility Layer - DUPLICATED"
        UTIL_FILES[util/files/<br/>File Operations]
        UTIL_ERRORS[util/errors/<br/>Error Handling]
        UTIL_CRYPTO[util/crypto/<br/>Cryptography]
        
        FILES_SCATTERED[50+ os.ReadFile<br/>SCATTERED]
        ERRORS_MIXED[fmt.Errorf + StructuredError<br/>INCONSISTENT]
    end

    CMD --> CLUSTER_SVC
    CMD --> CONFIG_LEGACY
    CMD --> CONFIG_MGR
    CMD --> GITOPS
    CMD --> SOPS
    
    CLUSTER_SVC --> VAL_CONFIG
    
    CONFIG_MGR --> VAL_CONFIG
    CONFIG_BUILDER --> VAL_CONFIG
    
    GITOPS --> VAL_GITOPS
    SOPS --> VAL_SOPS
    
    CLUSTER_SVC --> DI_ROOT
    CLUSTER_SVC --> DI_SETUP
    
    GITOPS --> FILES_SCATTERED
    SOPS --> FILES_SCATTERED
    CONFIG_LEGACY --> FILES_SCATTERED
    
    style CONFIG_LEGACY fill:#ffcccc
    style DI_ROOT fill:#ffcccc
    style FILES_SCATTERED fill:#ffcccc
    style ERRORS_MIXED fill:#ffcccc
    style VAL_SERVICES fill:#ffcccc
```

### Current Architecture Issues

**Red Boxes = Problem Areas:**

1. **Config Management Fragmentation**
   - 2 different systems for configuration management (legacy + manager)
   - Duplicate path resolution logic

2. **Validation Scattered**
   - 15+ separate validator implementations
   - No unified error format

3. **DI Container Duplication**
   - Two initialization points (root.go + setup.go)
   - Inconsistent container access patterns

4. **File Operations Scattered**
   - 50+ direct `os.ReadFile`/`os.WriteFile` calls
   - No centralized error handling
   - Inconsistent permission handling

---

## Proposed Architecture (To-Be)

```mermaid
graph TB
    subgraph "CLI Layer"
        CMD[cmd/root.go<br/>Command Handlers]
        CLUSTER[cmd/cluster_*.go<br/>Cluster Commands]
        CONFIG_CMD[cmd/config_*.go<br/>Config Commands]
        SOPS_CMD[cmd/sops_*.go<br/>SOPS Commands]
    end

    subgraph "Service Layer - UNIFIED"
        CLUSTER_SVC[internal/cluster/<br/>Cluster Services]
        
        subgraph "Config Management - SINGLE SYSTEM"
            CONFIG_MGR_NEW[ConfigurationManager<br/>Unified API]
            CONFIG_BUILDER_NEW[FluentConfigBuilder<br/>Builder Pattern]
            CONFIG_LOADER[ConfigLoader<br/>I/O Operations]
        end
        
        subgraph "Validation - CENTRALIZED"
            VAL_ENGINE[ValidationEngine<br/>Unified Registry]
            VAL_RULES[Validator Rules<br/>Pluggable]
        end
        
        GITOPS[internal/gitops/<br/>GitOps Generation]
        SOPS[internal/sops/<br/>Secrets Management]
    end

    subgraph "Core Infrastructure"
        DI_SINGLE[di/container.go<br/>Single DI System]
        
        CLOUD[cloud/factory.go<br/>Provider Factory]
        PROVISION[provision/<br/>Provisioning]
    end

    subgraph "Utility Layer - CONSOLIDATED"
        FS_WRAPPER[fs/wrapper.go<br/>Unified File Operations]
        ERROR_HANDLER[errors/handler.go<br/>Structured Errors]
        CRYPTO[crypto/manager.go<br/>Cryptography]
    end

    CMD --> CLUSTER_SVC
    CMD --> CONFIG_MGR_NEW
    CMD --> GITOPS
    CMD --> SOPS
    
    CLUSTER_SVC --> VAL_ENGINE
    CONFIG_MGR_NEW --> VAL_ENGINE
    CONFIG_MGR_NEW --> CONFIG_BUILDER_NEW
    CONFIG_MGR_NEW --> CONFIG_LOADER
    
    GITOPS --> VAL_ENGINE
    SOPS --> VAL_ENGINE
    
    CLUSTER_SVC --> DI_SINGLE
    CONFIG_MGR_NEW --> DI_SINGLE
    
    GITOPS --> FS_WRAPPER
    SOPS --> FS_WRAPPER
    CONFIG_LOADER --> FS_WRAPPER
    
    VAL_ENGINE --> ERROR_HANDLER
    FS_WRAPPER --> ERROR_HANDLER
    
    style CONFIG_MGR_NEW fill:#ccffcc
    style VAL_ENGINE fill:#ccffcc
    style DI_SINGLE fill:#ccffcc
    style FS_WRAPPER fill:#ccffcc
    style ERROR_HANDLER fill:#ccffcc
```

### Proposed Architecture Benefits

**Green Boxes = Improved Areas:**

1. **Unified Config Management**
   - Single `ConfigurationManager` with clear API
   - Builder pattern for construction
   - Centralized loading/saving

2. **Centralized Validation**
   - Single `ValidationEngine` with pluggable rules
   - Consistent error format
   - Reusable validators

3. **Single DI System**
   - One initialization point
   - Clear dependency graph
   - Better testability

4. **Consolidated Utilities**
   - Unified file operations wrapper
   - Structured error handling
   - Consistent patterns

---

## Component Interaction Flow

### Current Flow (Complex)

```mermaid
sequenceDiagram
    participant CLI as CLI Command
    participant Root as cmd/root.go
    participant Legacy as config.go
    participant Manager as ConfigManager
    participant Val1 as config/validator
    participant Val2 as core/validation
    
    CLI->>Root: Execute command
    Root->>Root: Initialize DI (duplicate)
    Root->>Legacy: Load config (legacy)
    Legacy->>Val1: Validate (old system)
    CLI->>Manager: Load config (new system)
    Manager->>Val2: Validate (new system)
    Val2-->>Manager: Partial validation
    Manager->>Val1: Fallback validation
    Val1-->>Manager: Complete validation
    Manager-->>CLI: Config with mixed validation
```

### Proposed Flow (Simplified)

```mermaid
sequenceDiagram
    participant CLI as CLI Command
    participant Root as cmd/root.go
    participant Manager as ConfigManager
    participant Engine as ValidationEngine
    participant Loader as ConfigLoader
    
    CLI->>Root: Execute command
    Root->>Root: Initialize DI (single)
    Root->>Manager: Get config
    Manager->>Loader: Load from file
    Loader-->>Manager: Raw config
    Manager->>Engine: Validate
    Engine-->>Manager: Validation result
    Manager-->>CLI: Validated config
```

---

## Migration Path Visualization

```mermaid
graph LR
    subgraph "Phase 1: Validation"
        A1[Current: 15+ validators] --> B1[Unified: ValidationEngine]
    end
    
    subgraph "Phase 2: Configuration"
        A2[Current: 3 systems] --> B2[Unified: ConfigManager]
    end
    
    subgraph "Phase 3: Utilities"
        A3[Current: Scattered] --> B3[Unified: Wrappers]
    end
    
    subgraph "Phase 4: Cleanup"
        A4[Current: Orphaned code] --> B4[Clean: Removed]
    end
    
    B1 --> A2
    B2 --> A3
    B3 --> A4
    
    style B1 fill:#ccffcc
    style B2 fill:#ccffcc
    style B3 fill:#ccffcc
    style B4 fill:#ccffcc
```

---

## Dependency Graph Simplification

### Before (Complex Dependencies)

```
cmd/root.go
├── internal/config/config.go (legacy)
├── internal/config/manager.go (new)
├── internal/config/builder.go (builder)
├── internal/di/setup.go (duplicate init)
└── internal/di/container.go

internal/cluster/
├── internal/config/validator.go
└── internal/config/manager.go

internal/gitops/
├── internal/gitops/validators.go
├── os.ReadFile (50+ calls)
└── fmt.Errorf (inconsistent)
```

### After (Clean Dependencies)

```
cmd/root.go
├── internal/config/manager.go (unified)
└── internal/di/container.go (single)

internal/cluster/
├── internal/config/manager.go
└── internal/core/validation/engine.go

internal/gitops/
├── internal/core/validation/engine.go
├── internal/util/fs/wrapper.go
└── internal/util/errors/handler.go
```

---

## Key Metrics Comparison

| Metric | Current | Proposed | Improvement |
|--------|---------|----------|-------------|
| Config Systems | 2 | 1 | 50% reduction |
| Validator Files | 15+ | 1 engine + rules | 80% reduction |
| DI Init Points | 2 | 1 | 50% reduction |
| File Op Calls | 50+ scattered | 1 wrapper | 98% reduction |
| Error Patterns | 3 mixed | 1 structured | 67% reduction |
| LOC (internal/) | ~45,000 | ~34,000 | 25% reduction |

---

## Architecture Decision Records (ADRs)

### ADR-001: Unified Validation Engine
**Status:** Proposed  
**Decision:** Migrate all validation to `internal/core/validation.ValidationEngine`  
**Rationale:** Eliminate duplication, consistent error handling, easier testing

### ADR-002: Single Configuration Manager
**Status:** Proposed  
**Decision:** Consolidate to `internal/config.ConfigurationManager`  
**Rationale:** Clear API, better caching, reduced complexity

### ADR-003: Centralized File Operations
**Status:** Proposed  
**Decision:** Create `internal/util/fs.Wrapper` for all file I/O  
**Rationale:** Consistent error handling, easier mocking, atomic operations

### ADR-004: Structured Error Handling
**Status:** Proposed  
**Decision:** Use `internal/util/errors.StructuredError` everywhere  
**Rationale:** Better error context, consistent formatting, easier debugging
