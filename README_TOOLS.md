# vsp Tool Reference

Complete documentation for all 68 MCP tools available in vsp.

**Mode Legend:**
- **Focused** - Available in focused mode (31 tools, default)
- **Expert** - Only available in expert mode (68 tools total)

---

## Unified Tools (2 tools) - Focused Mode

These tools replace 11 granular read/write operations with intelligent parameter-based routing:

| Tool | Description | Mode |
|------|-------------|------|
| `GetSource` | Unified read for any ABAP source. Parameters: `type` (PROG/CLAS/INTF/FUNC/FUGR/INCL/DDLS/MSAG), `name`, optional `parent` (for FUNC), optional `include` (for CLAS). | Focused |
| `WriteSource` | Unified write with auto-upsert. Parameters: `type`, `name`, `source`, `mode`, `options`. | Focused |

**Benefits:** 70% token reduction, simplified tool selection, extensible for new types.

---

## Search & Grep Tools (4 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `SearchObject` | Quick search for ABAP objects by name pattern | Focused |
| `GrepObjects` | Regex search across multiple objects (array of URLs) | Focused |
| `GrepPackages` | Regex search across packages with recursive subpackage support | Focused |
| `GrepObject` | Regex search in single object | Expert |
| `GrepPackage` | Regex search in single package | Expert |

**Grep Features:**
- Full regex support (Go regexp syntax)
- Case-sensitive or case-insensitive matching
- Context lines (like `grep -C`)
- Object type filtering
- Max results limit

---

## Read Operations (15 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `GetProgram` | Get ABAP program source | Expert |
| `GetClass` | Get ABAP class source | Expert |
| `GetInterface` | Get ABAP interface source | Expert |
| `GetFunction` | Get function module source | Expert |
| `GetFunctionGroup` | Get function group structure | Focused |
| `GetInclude` | Get ABAP include source | Expert |
| `GetTable` | Get table structure definition | Focused |
| `GetTableContents` | Get table data (supports SQL filtering) | Focused |
| `GetStructure` | Get structure definition | Expert |
| `GetPackage` | Get package contents | Focused |
| `GetTransaction` | Get transaction details | Expert |
| `GetTypeInfo` | Get data type information | Expert |
| `GetCDSDependencies` | Get CDS view dependency tree | Focused |
| `RunQuery` | Execute freestyle SQL query | Focused |

---

## System Information (2 tools) - NEW

| Tool | Description | Mode |
|------|-------------|------|
| `GetSystemInfo` | Get SAP system information (SID, release, kernel, database) | Focused |
| `GetInstalledComponents` | List installed software components with versions | Focused |

---

## Code Analysis (2 tools) - NEW

| Tool | Description | Mode |
|------|-------------|------|
| `GetCallGraph` | Get call hierarchy (callers/callees) for methods/functions | Focused |
| `GetObjectStructure` | Get object explorer tree structure | Focused |

---

## Development Tools (4 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `SyntaxCheck` | Check source code for syntax errors | Focused |
| `Activate` | Activate an ABAP object | Expert |
| `RunUnitTests` | Execute ABAP Unit tests | Focused |
| `RunATCCheck` | Run ATC code quality checks | Focused |

---

## ATC (Code Quality) Tools (2 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `RunATCCheck` | Run ATC check, returns findings with priority (1=Error, 2=Warning, 3=Info) | Focused |
| `GetATCCustomizing` | Get ATC system configuration | Expert |

**Example ATC Output:**
```json
{
  "summary": { "totalFindings": 3, "errors": 1, "warnings": 2 },
  "worklist": {
    "objects": [{
      "name": "ZCL_TEST",
      "findings": [{ "priority": 1, "checkTitle": "Syntax Check", "line": 42 }]
    }]
  }
}
```

---

## CRUD Operations (5 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `LockObject` | Acquire edit lock | Focused |
| `UnlockObject` | Release edit lock | Focused |
| `CreateObject` | Create new object (program, class, interface, include, function group, function module, package) | Expert |
| `UpdateSource` | Write source code | Expert |
| `DeleteObject` | Delete an object | Expert |

---

## Class Include Operations (3 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `GetClassInclude` | Get class include (definitions, implementations, macros, testclasses) | Expert |
| `CreateTestInclude` | Create test classes include | Expert |
| `UpdateClassInclude` | Update class include source | Expert |

---

## Workflow Tools (5 tools)

Composite operations that combine multiple ADT API calls:

| Tool | Description | Steps | Mode |
|------|-------------|-------|------|
| `EditSource` | **Surgical string replacement** (matches Edit tool pattern) | GetSource → FindReplace → SyntaxCheck → Lock → Update → Unlock → Activate | Focused |
| `WriteProgram` | Update program with activation | Lock → SyntaxCheck → Update → Unlock → Activate | Expert |
| `WriteClass` | Update class with activation | Lock → SyntaxCheck → Update → Unlock → Activate | Expert |
| `CreateAndActivateProgram` | Create new program | Create → UpdateSource → Activate | Expert |
| `CreateClassWithTests` | Create class with unit tests | Create → Lock → Update → CreateTestInclude → WriteTests → Unlock → Activate → RunUnitTests | Expert |

---

## File-Based Deployment Tools (5 tools)

Solves token limit problem for large files:

| Tool | Description | Mode |
|------|-------------|------|
| `ImportFromFile` | **File → SAP** - Smart deploy with auto create/update detection | Focused |
| `ExportToFile` | **SAP → File** - Save object source to local file | Focused |
| `DeployFromFile` | Legacy name for ImportFromFile | Expert |
| `SaveToFile` | Legacy name for ExportToFile | Expert |
| `RenameObject` | Rename object by creating copy | Expert |

**Supported Extensions:**
- `.clas.abap` - Classes
- `.prog.abap` - Programs
- `.intf.abap` - Interfaces
- `.fugr.abap` - Function Groups
- `.func.abap` - Function Modules

---

## Code Intelligence Tools (7 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `FindDefinition` | Navigate to symbol definition | Focused |
| `FindReferences` | Find all references to symbol | Focused |
| `CodeCompletion` | Get code completion suggestions | Expert |
| `PrettyPrint` | Format ABAP source code | Expert |
| `GetPrettyPrinterSettings` | Get formatter settings | Expert |
| `SetPrettyPrinterSettings` | Update formatter settings | Expert |
| `GetTypeHierarchy` | Get type hierarchy (supertypes/subtypes) | Expert |

---

## Transport Tools (3 tools)

| Tool | Description | Mode |
|------|-------------|------|
| `CreateTransport` | Create transport request | Expert |
| `GetTransportInfo` | Get transport details | Expert |
| `ReleaseTransport` | Release transport | Expert |
| `GetUserTransports` | List user's transports | Expert |
| `GetInactiveObjects` | List inactive objects | Expert |

---

## ExecuteABAP (Expert Mode)

Execute arbitrary ABAP code via unit test wrapper:

| Tool | Description | Mode |
|------|-------------|------|
| `ExecuteABAP` | Run ABAP code and capture output | Expert |

**Risk Levels:**
- `harmless` - Read-only, no external calls
- `dangerous` - Can write to DB, call external
- `critical` - Full system access

See [ExecuteABAP Implementation Report](reports/2025-12-05-004-execute-abap-implementation.md) for details.

---

## Runtime Errors / Short Dumps (2 tools) - RABAX

| Tool | Description | Mode |
|------|-------------|------|
| `GetDumps` | List runtime errors with filters (user, exception type, program, date range) | Focused |
| `GetDump` | Get full details of a specific dump including stack trace | Focused |

**Use Cases:**
- Monitor system health by checking recent dumps
- Debug production issues by examining dump details
- Track error patterns by exception type

---

## ABAP Profiler / Traces (2 tools) - ATRA

| Tool | Description | Mode |
|------|-------------|------|
| `ListTraces` | List ABAP runtime traces (profiler results) | Focused |
| `GetTrace` | Get trace analysis (hitlist, statements, dbAccesses) | Focused |

**Analysis Types:**
- `hitlist` - Hot spots by execution time
- `statements` - Statement-level trace
- `dbAccesses` - Database access analysis

---

## SQL Trace (2 tools) - ST05

| Tool | Description | Mode |
|------|-------------|------|
| `GetSQLTraceState` | Check if SQL trace is currently active | Focused |
| `ListSQLTraces` | List SQL trace files | Focused |

---

## Tool Count Summary

| Mode | Tools | Description |
|------|-------|-------------|
| **Focused** | 31 | Essential tools for AI-assisted development |
| **Expert** | 68 | All tools including low-level operations |

**Token Savings with Focused Mode:**
- Tool definitions: 69% reduction (~6,500 → ~2,000 tokens)
- Typical workflow: 73% reduction
- Decision clarity: 31 choices instead of 68
