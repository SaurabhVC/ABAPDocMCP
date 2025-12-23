# ZADT_VSP Self-Deployment Design

**Date:** 2025-12-23
**Report ID:** 004
**Subject:** Deploy ABAP components via ADT/MCP for one-command setup
**Status:** Design

---

## Executive Summary

**Goal:** Enable one-command deployment of the ZADT_VSP WebSocket handler to any SAP system:

```bash
vsp install zadt-vsp
```

This unlocks:
- **Debugging** - WebSocket TPDAPI integration (step, inspect, attach)
- **Git Export** - abapGit-compatible export (158 object types)
- **RFC Calls** - Execute any function module with parameters
- **AMDP Debugging** - HANA stored procedure debugging (experimental)

---

## Current State

### ABAP Objects (Embedded in Binary)

| File | Object | Type | Dependencies |
|------|--------|------|--------------|
| `zif_vsp_service.intf.abap` | `ZIF_VSP_SERVICE` | INTF | None |
| `zcl_vsp_rfc_service.clas.abap` | `ZCL_VSP_RFC_SERVICE` | CLAS | ZIF_VSP_SERVICE |
| `zcl_vsp_debug_service.clas.abap` | `ZCL_VSP_DEBUG_SERVICE` | CLAS | ZIF_VSP_SERVICE, CL_TPDAPI_* |
| `zcl_vsp_amdp_service.clas.abap` | `ZCL_VSP_AMDP_SERVICE` | CLAS | ZIF_VSP_SERVICE, CL_AMDP_* |
| `zcl_vsp_git_service.clas.abap` | `ZCL_VSP_GIT_SERVICE` | CLAS | ZIF_VSP_SERVICE, ZCL_ABAPGIT_* |
| `zcl_vsp_apc_handler.clas.abap` | `ZCL_VSP_APC_HANDLER` | CLAS | ZIF_VSP_SERVICE, CL_APC_WSP_* |

### Deployment Order (Dependencies)

```
1. ZIF_VSP_SERVICE          (interface - no dependencies)
2. ZCL_VSP_RFC_SERVICE      (depends on interface)
3. ZCL_VSP_DEBUG_SERVICE    (depends on interface)
4. ZCL_VSP_AMDP_SERVICE     (depends on interface)
5. ZCL_VSP_GIT_SERVICE      (depends on interface, optional abapGit)
6. ZCL_VSP_APC_HANDLER      (depends on interface + all services)
```

---

## Proposed Implementation

### Option A: MCP Tool (Recommended)

Add new MCP tool `InstallZADTVSP`:

```go
// internal/mcp/server.go

case "InstallZADTVSP":
    result, err := s.installZADTVSP(ctx, args)
    if err != nil {
        return newToolResultError(err.Error()), nil
    }
    return newToolResultText(result), nil
```

**Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `package` | string | `$ZADT_VSP` | Target package name |
| `skip_git_service` | bool | false | Skip Git service (if no abapGit) |
| `check_only` | bool | false | Only check prerequisites |

### Option B: CLI Command

Add new CLI command:

```bash
vsp install zadt-vsp [--package $PKG] [--skip-git-service] [--check-only]
```

---

## Deployment Workflow

### Phase 1: Prerequisites Check

```go
func (s *Server) checkZADTVSPPrerequisites(ctx context.Context) (*PrereqResult, error) {
    result := &PrereqResult{}

    // 1. Check if package exists (or can be created)
    pkg, err := s.client.GetPackage(ctx, "$ZADT_VSP")
    if err != nil {
        result.NeedsPackageCreation = true
    }

    // 2. Check existing objects
    existing := []string{}
    for _, obj := range []string{"ZIF_VSP_SERVICE", "ZCL_VSP_APC_HANDLER", ...} {
        _, err := s.client.SearchObject(ctx, obj, 1)
        if err == nil {
            existing = append(existing, obj)
        }
    }
    result.ExistingObjects = existing

    // 3. Check abapGit availability (for Git service)
    _, err = s.client.SearchObject(ctx, "ZCL_ABAPGIT_OBJECTS", 1)
    result.HasAbapGit = (err == nil)

    // 4. Check APC availability
    result.HasAPC = s.client.HasFeature(ctx, "APC")

    return result, nil
}
```

### Phase 2: Create Package

```go
func (s *Server) createZADTVSPPackage(ctx context.Context) error {
    return s.client.CreatePackage(ctx, "$ZADT_VSP", "VSP WebSocket Handler")
}
```

### Phase 3: Deploy Objects

```go
func (s *Server) deployZADTVSPObjects(ctx context.Context, skipGitService bool) (*DeployResult, error) {
    objects := []struct {
        Type   string
        Name   string
        Source string
    }{
        {"INTF", "ZIF_VSP_SERVICE", embedded.ZifVspService},
        {"CLAS", "ZCL_VSP_RFC_SERVICE", embedded.ZclVspRfcService},
        {"CLAS", "ZCL_VSP_DEBUG_SERVICE", embedded.ZclVspDebugService},
        {"CLAS", "ZCL_VSP_AMDP_SERVICE", embedded.ZclVspAmdpService},
        {"CLAS", "ZCL_VSP_GIT_SERVICE", embedded.ZclVspGitService},  // Skip if no abapGit
        {"CLAS", "ZCL_VSP_APC_HANDLER", embedded.ZclVspApcHandler},
    }

    result := &DeployResult{}
    for _, obj := range objects {
        if obj.Name == "ZCL_VSP_GIT_SERVICE" && skipGitService {
            result.Skipped = append(result.Skipped, obj.Name)
            continue
        }

        err := s.client.WriteSource(ctx, obj.Type, obj.Name, obj.Source, "$ZADT_VSP")
        if err != nil {
            result.Failed = append(result.Failed, obj.Name+": "+err.Error())
        } else {
            result.Deployed = append(result.Deployed, obj.Name)
        }
    }

    return result, nil
}
```

### Phase 4: Generate Post-Deployment Instructions

```go
func generatePostDeployInstructions() string {
    return `
## Post-Deployment Steps (Manual in SAP GUI)

### 1. Create APC Application (Transaction SAPC)

1. Start transaction SAPC
2. Click "Create" button
3. Fill in:
   - **Application ID:** ZADT_VSP
   - **Description:** VSP WebSocket Handler
   - **Handler Class:** ZCL_VSP_APC_HANDLER
   - **State:** Stateful
4. Save and activate

### 2. Activate ICF Service (Transaction SICF)

1. Start transaction SICF
2. Navigate to: /sap/bc/apc/sap/zadt_vsp
3. Right-click the node → "Activate Service"
4. Confirm activation

### 3. Test Connection

Using wscat or browser:
` + "```" + `
wscat -c "ws://host:port/sap/bc/apc/sap/zadt_vsp?sap-client=001" \
      -H "Authorization: Basic $(echo -n user:pass | base64)"
` + "```" + `

Expected welcome message:
` + "```json" + `
{"id":"welcome","success":true,"data":{"session":"...","version":"2.2.0","domains":["rfc","debug","amdp","git"]}}
` + "```" + `

### 4. Verify in vsp

` + "```bash" + `
# Test RFC domain
vsp call-rfc RFC_SYSTEM_INFO

# Test Git domain (requires abapGit)
vsp git-types
` + "```" + `
`
}
```

---

## Embedding Source Files

### Option A: Go Embed (Recommended)

```go
// embedded/abap/embed.go

package embedded

import _ "embed"

//go:embed zif_vsp_service.intf.abap
var ZifVspService string

//go:embed zcl_vsp_rfc_service.clas.abap
var ZclVspRfcService string

//go:embed zcl_vsp_debug_service.clas.abap
var ZclVspDebugService string

//go:embed zcl_vsp_amdp_service.clas.abap
var ZclVspAmdpService string

//go:embed zcl_vsp_git_service.clas.abap
var ZclVspGitService string

//go:embed zcl_vsp_apc_handler.clas.abap
var ZclVspApcHandler string
```

### Option B: Generate Constants (Current Pattern)

If `//go:embed` isn't available, generate Go constants from source files during build.

---

## User Experience

### Successful Installation

```
$ vsp install zadt-vsp

Checking prerequisites...
  ✓ ADT connection OK
  ✓ Package creation available
  ✓ APC feature available
  ✓ abapGit detected → Git service will be deployed

Creating package $ZADT_VSP...
  ✓ Package created

Deploying ABAP objects...
  [1/6] ZIF_VSP_SERVICE    ✓ Created
  [2/6] ZCL_VSP_RFC_SERVICE    ✓ Created
  [3/6] ZCL_VSP_DEBUG_SERVICE  ✓ Created
  [4/6] ZCL_VSP_AMDP_SERVICE   ✓ Created
  [5/6] ZCL_VSP_GIT_SERVICE    ✓ Created
  [6/6] ZCL_VSP_APC_HANDLER    ✓ Created

═══════════════════════════════════════════════════════════════
  DEPLOYMENT COMPLETE - Manual Steps Required
═══════════════════════════════════════════════════════════════

The ABAP objects have been deployed. Complete the setup:

1. Transaction SAPC → Create APC Application
   - ID: ZADT_VSP
   - Handler: ZCL_VSP_APC_HANDLER
   - State: Stateful

2. Transaction SICF → Activate Service
   - Path: /sap/bc/apc/sap/zadt_vsp

3. Test: wscat -c "ws://host:port/sap/bc/apc/sap/zadt_vsp"

For detailed instructions, see:
  embedded/abap/README.md

Features unlocked:
  ✓ WebSocket debugging (TPDAPI)
  ✓ RFC/BAPI execution
  ✓ AMDP debugging (experimental)
  ✓ abapGit export (158 object types)
```

### Partial Installation (No abapGit)

```
$ vsp install zadt-vsp

Checking prerequisites...
  ✓ ADT connection OK
  ✓ Package creation available
  ✓ APC feature available
  ⚠ abapGit not detected → Git service will be skipped

Deploying ABAP objects...
  [1/5] ZIF_VSP_SERVICE    ✓ Created
  [2/5] ZCL_VSP_RFC_SERVICE    ✓ Created
  [3/5] ZCL_VSP_DEBUG_SERVICE  ✓ Created
  [4/5] ZCL_VSP_AMDP_SERVICE   ✓ Created
  [5/5] ZCL_VSP_APC_HANDLER    ✓ Created
  [-/-] ZCL_VSP_GIT_SERVICE    ⊘ Skipped (no abapGit)

Features unlocked:
  ✓ WebSocket debugging (TPDAPI)
  ✓ RFC/BAPI execution
  ✓ AMDP debugging (experimental)
  ✗ abapGit export (install abapGit first)
```

---

## Tool Group & Mode

- **Tool Group:** "I" (Install) - can be disabled with `--disabled-groups I`
- **Mode:** Available in both focused and expert modes
- **Permissions:** Requires write access to target package

---

## Safety Considerations

1. **Non-destructive:** Check if objects exist before overwriting
2. **Confirm prompt:** `--yes` flag to skip confirmation
3. **Dry-run:** `--check-only` to see what would happen
4. **Rollback info:** Log what was created for manual cleanup

---

## Future Enhancements

### 1. SAPC Auto-Creation

Investigate if APC applications can be created via ADT:
- `/sap/bc/adt/apcdiscovery` endpoint?
- OData service for SAPC?

### 2. SICF Auto-Activation

Investigate if ICF services can be activated programmatically:
- `/sap/bc/adt/vit/icfnodes` endpoint?

### 3. Uninstall Command

```bash
vsp uninstall zadt-vsp
```

Deletes all ZADT_VSP objects (with confirmation).

### 4. Upgrade Command

```bash
vsp upgrade zadt-vsp
```

Updates existing objects to latest version.

---

## Implementation Plan

| Phase | Task | Effort |
|-------|------|--------|
| 1 | Create `embedded/abap/embed.go` with go:embed | 30 min |
| 2 | Add `InstallZADTVSP` tool handler | 2 hours |
| 3 | Add prerequisite checking | 1 hour |
| 4 | Add CLI command wrapper | 30 min |
| 5 | Test on real system | 1 hour |
| 6 | Documentation | 30 min |

**Total:** ~5-6 hours

---

## Related Documents

- [embedded/abap/README.md](../embedded/abap/README.md) - Manual deployment instructions
- [reports/2025-12-18-002-websocket-rfc-handler.md](./2025-12-18-002-websocket-rfc-handler.md) - WebSocket handler design
- [reports/2025-12-23-002-abapgit-websocket-integration-complete.md](./2025-12-23-002-abapgit-websocket-integration-complete.md) - Git service implementation

---

## Dependency Embedding Architecture (Implemented)

### Hybrid Approach

We use a hybrid approach for embedding:

1. **Our code (ZADT_VSP):** Raw ABAP strings via `go:embed` - self-contained, no dependencies
2. **Dependencies (abapGit):** abapGit ZIP format - standard container, unzip + deploy via WriteSource

### Directory Structure

```
embedded/
├── abap/                         # OUR CODE (raw ABAP)
│   ├── zif_vsp_service.intf.abap
│   ├── zcl_vsp_*.clas.abap
│   └── embed.go                  # go:embed as strings
│
└── deps/                         # DEPENDENCIES (abapGit ZIP)
    ├── abapgit-standalone.zip    # Full abapGit (placeholder)
    ├── abapgit-dev.zip           # Developer edition (placeholder)
    └── embed.go                  # Unzip + deploy logic
```

### deps/embed.go Features

- **`ParseAbapGitFilename()`** - Extracts object type, name, include type from abapGit filenames
- **`UnzipInMemory()`** - Extracts all files from ZIP without touching disk
- **`DeploymentOrder()`** - Sorts files by dependency (INTF → CLAS → PROG → etc.)
- **`CreateDeploymentPlan()`** - Groups files by object, extracts descriptions from XML

### abapGit File Parsing

| Filename | ObjectType | ObjectName | IncludeType |
|----------|------------|------------|-------------|
| `zcl_example.clas.abap` | CLAS | ZCL_EXAMPLE | (main) |
| `zcl_example.clas.locals_def.abap` | CLAS | ZCL_EXAMPLE | locals_def |
| `zcl_example.clas.locals_imp.abap` | CLAS | ZCL_EXAMPLE | locals_imp |
| `zcl_example.clas.testclasses.abap` | CLAS | ZCL_EXAMPLE | testclasses |
| `zif_example.intf.abap` | INTF | ZIF_EXAMPLE | (main) |
| `zexample.ddls.asddls` | DDLS | ZEXAMPLE | (main) |

### MCP Tools

| Tool | Description |
|------|-------------|
| `InstallZADTVSP` | Deploy ZADT_VSP from embedded raw ABAP |
| `InstallAbapGit` | Deploy abapGit from embedded ZIP |
| `ListDependencies` | Show available dependencies |

### Benefits

1. **Bootstrap capability** - VSP can install its own dependencies
2. **Self-contained** - Single binary contains everything
3. **Standard format** - abapGit ZIP is widely used
4. **No target dependencies** - WriteSource works on any ADT system

---

## Conclusion

The `InstallZADTVSP` tool will:

1. **Simplify adoption** - One command vs. manual copy/paste
2. **Handle dependencies** - Correct deployment order
3. **Detect capabilities** - Skip Git service if no abapGit
4. **Guide completion** - Clear post-deployment instructions

This makes ZADT_VSP as easy to deploy as any other MCP tool operation.
