# mcp-abap-adt-go

A Go-native MCP (Model Context Protocol) server for SAP ABAP Development Tools (ADT).

Single-binary distribution of 36 ADT tools for use with Claude and other MCP-compatible AI assistants.

## Why This Project?

This project brings **AI-assisted ABAP development** to Claude Code and Claude Desktop by exposing SAP ADT capabilities through the Model Context Protocol. With this MCP server, Claude can:

- Read and understand your ABAP codebase
- Create, modify, and delete ABAP objects
- Run syntax checks and unit tests
- Navigate code with find definition/references
- Format code with pretty printer
- Execute SQL queries against SAP tables

## Inspirations & Credits

This project stands on the shoulders of giants:

| Project | Author | Description |
|---------|--------|-------------|
| [abap-adt-api](https://github.com/marcellourbani/abap-adt-api) | Marcello Urbani | TypeScript library that implements the ADT REST API. Powers the [ABAP Remote FS](https://github.com/marcellourbani/vscode_abap_remote_fs) VS Code extension. The definitive reference for ADT API implementation. |
| [mcp-abap-adt](https://github.com/mario-andrle/mcp-abap-adt) | Mario Andrle | First MCP server for ABAP ADT in TypeScript/Node.js. Pioneered the concept of AI-assisted ABAP development via MCP. |

**mcp-abap-adt-go** is a complete rewrite in Go, providing:
- Single binary with zero runtime dependencies
- Extended toolset (36 vs 13 tools)
- Full CRUD operations and code intelligence
- ~50x faster startup time

## Capability Matrix

Comparison of ADT capabilities across implementations:

| Capability | ADT (Eclipse) | abap-adt-api (TS) | mcp-abap-adt (TS) | **mcp-abap-adt-go** |
|------------|:-------------:|:-----------------:|:-----------------:|:-------------------:|
| **Source Read** |
| Programs, Classes, Interfaces | Y | Y | Y | **Y** |
| Functions, Function Groups | Y | Y | Y | **Y** |
| Tables, Structures | Y | Y | Y | **Y** |
| Includes | Y | Y | Y | **Y** |
| Package Contents | Y | Y | Y | **Y** |
| Type Info | Y | Y | P | **Y** |
| CDS Views | Y | Y | N | N |
| RAP/BDEF | Y | Y | N | N |
| **Data Query** |
| Table Contents | Y | Y | P | **Y** |
| SQL Filtering | Y | Y | N | **Y** |
| Freestyle SQL | Y | Y | N | **Y** |
| **Development Tools** |
| Syntax Check | Y | Y | N | **Y** |
| Activation | Y | Y | N | **Y** |
| Unit Tests | Y | Y | N | **Y** |
| **CRUD Operations** |
| Lock/Unlock | Y | Y | N | **Y** |
| Create Objects | Y | Y | N | **Y** |
| Update Source | Y | Y | N | **Y** |
| Delete Objects | Y | Y | N | **Y** |
| Class Includes | Y | Y | N | **Y** |
| **Code Intelligence** |
| Find Definition | Y | Y | N | **Y** |
| Find References | Y | Y | N | **Y** |
| Code Completion | Y | Y | N | **Y** |
| Type Hierarchy | Y | Y | N | **Y** |
| Pretty Printer | Y | Y | N | **Y** |
| **Workflow Tools** |
| Write & Activate | - | - | N | **Y** |
| Create & Activate | - | - | N | **Y** |
| Create with Tests | - | - | N | **Y** |
| **Transports** |
| Transport Management | Y | Y | N | N |
| **ATC** |
| Code Quality Checks | Y | Y | N | N |
| **Debugging** |
| Remote Debugging | Y | Y | N | N |

**Legend:** Y = Full support, P = Partial, N = Not implemented, - = Not applicable

## Available Tools (36)

### Read Operations (14 tools)

| Tool | Description |
|------|-------------|
| `SearchObject` | Search for ABAP objects |
| `GetProgram` | Retrieve ABAP program source code |
| `GetClass` | Retrieve ABAP class source code |
| `GetInterface` | Retrieve ABAP interface source code |
| `GetFunction` | Retrieve function module source code |
| `GetFunctionGroup` | Retrieve function group structure |
| `GetInclude` | Retrieve ABAP include source code |
| `GetTable` | Retrieve ABAP table structure |
| `GetTableContents` | Retrieve data from ABAP table (supports SQL filtering) |
| `GetStructure` | Retrieve ABAP structure definition |
| `GetPackage` | Retrieve package contents |
| `GetTransaction` | Retrieve transaction details |
| `GetTypeInfo` | Retrieve data type information |
| `RunQuery` | Execute freestyle SQL query |

### Development Tools (3 tools)

| Tool | Description |
|------|-------------|
| `SyntaxCheck` | Check ABAP source code for syntax errors |
| `Activate` | Activate an ABAP object |
| `RunUnitTests` | Execute ABAP Unit tests |

### CRUD Operations (5 tools)

| Tool | Description |
|------|-------------|
| `LockObject` | Acquire edit lock on an ABAP object |
| `UnlockObject` | Release edit lock |
| `CreateObject` | Create new ABAP object (program, class, interface, include, function group, function module) |
| `UpdateSource` | Write source code to an object |
| `DeleteObject` | Delete an ABAP object |

### Class Include Operations (3 tools)

| Tool | Description |
|------|-------------|
| `GetClassInclude` | Retrieve class include source (definitions, implementations, macros, testclasses) |
| `CreateTestInclude` | Create test classes include for a class |
| `UpdateClassInclude` | Update class include source |

### Workflow Tools (4 tools)

| Tool | Description |
|------|-------------|
| `WriteProgram` | Update existing program with syntax check and activation |
| `WriteClass` | Update existing class with syntax check and activation |
| `CreateAndActivateProgram` | Create new program with source and activate it |
| `CreateClassWithTests` | Create class with unit tests and run them |

### Code Intelligence Tools (7 tools)

| Tool | Description |
|------|-------------|
| `FindDefinition` | Navigate to symbol definition |
| `FindReferences` | Find all references to an object or symbol |
| `CodeCompletion` | Get code completion suggestions |
| `PrettyPrint` | Format ABAP source code |
| `GetPrettyPrinterSettings` | Get formatter settings |
| `SetPrettyPrinterSettings` | Update formatter settings |
| `GetTypeHierarchy` | Get type hierarchy (supertypes/subtypes) |

## Installation

### Pre-built Binaries

Download from the [releases page](https://github.com/oisee/vibing-steamer/releases).

Available for:
- Linux (amd64, arm64, 386, arm)
- macOS (amd64, arm64/Apple Silicon)
- Windows (amd64, arm64, 386)

### From Source

```bash
git clone https://github.com/oisee/vibing-steamer.git
cd vibing-steamer

# Build for current platform
make build

# Build for all platforms
make build-all
```

## Configuration

The server is configured via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SAP_URL` | Yes | - | SAP system URL (e.g., `https://host:44300`) |
| `SAP_USER` | Yes | - | SAP username |
| `SAP_PASSWORD` | Yes | - | SAP password |
| `SAP_CLIENT` | No | `001` | SAP client number |
| `SAP_LANGUAGE` | No | `EN` | SAP language |
| `SAP_INSECURE` | No | `false` | Skip TLS certificate verification |

## Usage with Claude Desktop

Add to your Claude Desktop configuration (`~/.config/claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "abap-adt": {
      "command": "/path/to/mcp-abap-adt-go",
      "env": {
        "SAP_URL": "https://your-sap-host:44300",
        "SAP_USER": "your-username",
        "SAP_PASSWORD": "your-password",
        "SAP_CLIENT": "001",
        "SAP_LANGUAGE": "EN"
      }
    }
  }
}
```

## Usage with Claude Code

Add `.mcp.json` to your project root:

```json
{
  "mcpServers": {
    "abap-adt": {
      "command": "/path/to/mcp-abap-adt-go",
      "env": {
        "SAP_URL": "https://your-sap-host:44300",
        "SAP_USER": "your-username",
        "SAP_PASSWORD": "your-password",
        "SAP_CLIENT": "001",
        "SAP_LANGUAGE": "EN"
      }
    }
  }
}
```

## Development

```bash
# Run unit tests
go test ./...

# Run integration tests (requires SAP system)
SAP_URL=http://host:port SAP_USER=user SAP_PASSWORD=pass \
  go test -tags=integration -v ./pkg/adt/

# Build for current platform
make build

# Build for all platforms
make build-all

# Build for specific OS
make build-linux
make build-darwin
make build-windows
```

## Architecture

```
mcp-abap-adt-go/
├── cmd/mcp-abap-adt-go/     # Main entry point
│   └── main.go              # MCP server startup
├── pkg/adt/                 # ADT client library
│   ├── client.go            # Main client facade + read operations
│   ├── config.go            # Configuration from environment
│   ├── http.go              # HTTP transport with CSRF & session handling
│   ├── crud.go              # CRUD operations (lock, unlock, create, update, delete)
│   ├── devtools.go          # Development tools (syntax check, activate, unit tests)
│   ├── codeintel.go         # Code intelligence (definition, references, completion)
│   ├── workflows.go         # High-level workflow operations
│   └── xml.go               # XML parsing utilities
├── internal/mcp/            # MCP server implementation
│   └── server.go            # Tool registration and handlers (36 tools)
├── reports/                 # Project documentation and status
└── testdata/                # Test fixtures
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architecture documentation.

## Comparison Summary

| Aspect | mcp-abap-adt (TS) | mcp-abap-adt-go |
|--------|-------------------|-----------------|
| **Tools** | 13 | **36** |
| **Language** | TypeScript | Go |
| **Runtime** | Node.js required | Single binary |
| **Distribution** | npm install | Download & run |
| **Startup Time** | ~500ms | **~10ms** |
| **SQL Queries** | No | **Yes** |
| **Syntax Check** | No | **Yes** |
| **Unit Tests** | No | **Yes** |
| **Activation** | No | **Yes** |
| **CRUD Operations** | No | **Yes** |
| **Class Includes** | No | **Yes** |
| **Code Intelligence** | No | **Yes** |
| **Workflow Tools** | No | **Yes** |

## Roadmap

- [ ] Transport Management
- [ ] ATC (Code Quality) Integration
- [ ] CDS View Support
- [ ] RAP/BDEF Support

## License

MIT

## Contributing

Contributions are welcome! Please see [ARCHITECTURE.md](ARCHITECTURE.md) and [CLAUDE.md](CLAUDE.md) for development guidelines.
