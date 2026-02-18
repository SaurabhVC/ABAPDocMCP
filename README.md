# ABAPDocMCP — WRICEF Technical Specification Generator

> **MCP tool that generates structured ABAP Technical Specification documents from SAP Transport Requests.**
> Connects to any SAP system via ADT REST API and produces ready-to-review Markdown specs covering all WRICEF object types.

---

## What It Does

`GenerateWricefTechSpec` is an MCP (Model Context Protocol) tool that:

1. Reads one or more **Transport Request numbers** from your SAP system
2. Fetches the transport objects and their **source code** via ADT
3. Analyses the source for DB tables, function module calls, authorisation objects, error handling patterns, and performance anti-patterns
4. Produces a fully structured **Markdown Technical Specification** with up to 32 configurable sections
5. Includes a **Mermaid object-relationship diagram** with colour-coded code objects grouped by transport

---

## WRICEF Types Supported

| Type | Code | Dedicated Sections |
|------|------|--------------------|
| Report | `REPORT` | Selection Screen / Input Parameters, Output / ALV Layout |
| Interface | `INTERFACE` | Interface Details (direction, protocol, frequency), Message Structure / Field Mapping |
| Conversion | `CONVERSION` | Pseudocode, DB Objects, Error Handling |
| Enhancement | `ENHANCEMENT` | Enhancement Point Details (BAdI / User Exit / Implicit) |
| Form | `FORM` | Form Details (SmartForms / Adobe / SAPscript), Output Type |
| Workflow | `WORKFLOW` | Workflow Steps |
| OData / RAP | any type | OData / RAP Details (Service Binding, Service Definition) |

One WRICEF can span **multiple transport requests** and contain **mixed object types** — all are handled in a single spec.

---

## Quick Start

### 1. Prerequisites

- Go 1.21+ (build from source) **or** use a pre-built binary
- SAP system with ADT enabled (`/sap/bc/adt/` ICF services active)

### 2. Build

```bash
git clone https://github.com/SaurabhVC/ABAPDocMCP.git
cd ABAPDocMCP
go build -o abapdocmcp ./cmd/vsp/main.go
```

### 3. Configure Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "abapdocmcp": {
      "command": "/path/to/abapdocmcp",
      "env": {
        "SAP_URL": "https://your-sap-host:44300",
        "SAP_USER": "your-username",
        "SAP_PASSWORD": "your-password",
        "SAP_CLIENT": "100"
      }
    }
  }
}
```

> **Self-signed certificates:** Add `"SAP_INSECURE": "true"` to the env block.

### 4. Configure Claude Code

Add `.mcp.json` to your project root:

```json
{
  "mcpServers": {
    "abapdocmcp": {
      "command": "/path/to/abapdocmcp",
      "env": {
        "SAP_URL": "https://your-sap-host:44300",
        "SAP_USER": "your-username",
        "SAP_PASSWORD": "your-password",
        "SAP_CLIENT": "100"
      }
    }
  }
}
```

---

## Tool Reference — `GenerateWricefTechSpec`

### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `transport_numbers` | string | Comma-separated TR numbers, e.g. `DEVK900123,DEVK900124` |

### WRICEF Metadata (optional)

| Parameter | Type | Description |
|-----------|------|-------------|
| `wricef_type` | string | `REPORT` \| `INTERFACE` \| `CONVERSION` \| `ENHANCEMENT` \| `FORM` \| `WORKFLOW` (default: `REPORT`) |
| `wricef_id` | string | WRICEF identifier, e.g. `ZSD-R-001` |
| `complexity` | string | `S` \| `M` \| `L` \| `XL` |
| `cr_number` | string | Change request / ticket number |
| `module` | string | SAP module: `SD`, `FI`, `MM`, `HR`, … |

### People

| Parameter | Type | Description |
|-----------|------|-------------|
| `developer_name` | string | Developer full name |
| `developer_user_id` | string | Developer SAP user ID |
| `reviewer_name` | string | Reviewer full name |
| `approver_name` | string | Approver full name |

### System

| Parameter | Type | Description |
|-----------|------|-------------|
| `system_id` | string | SAP system ID (default: `DEV`) |
| `client` | string | SAP client (default: `100`) |
| `version` | string | Document version (default: `1.0`) |

### Source Code Enrichment

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `fetch_source_code` | boolean | `true` | Fetch and analyse source code for DB tables, FM calls, auth objects, error handling, performance patterns |

### Interface-Specific

| Parameter | Description |
|-----------|-------------|
| `interface_direction` | `Inbound` \| `Outbound` \| `Bidirectional` |
| `interface_protocol` | `RFC` \| `IDoc` \| `REST` \| `SOAP` \| `File` |
| `interface_frequency` | `Real-time` \| `Batch` \| `Event-driven` |

### Enhancement-Specific

| Parameter | Description |
|-----------|-------------|
| `enhancement_type` | `BAdI` \| `User Exit` \| `Implicit Enhancement` |
| `enhancement_name` | BAdI / enhancement exit name |
| `original_program` | Program or object being enhanced |

### Form-Specific

| Parameter | Description |
|-----------|-------------|
| `form_technology` | `SmartForms` \| `Adobe Forms` \| `SAPscript` |
| `form_output_type` | `Spool` \| `Email` \| `PDF` |

### Workflow-Specific

| Parameter | Description |
|-----------|-------------|
| `workflow_steps` | Workflow steps description |

### OData / RAP-Specific

| Parameter | Description |
|-----------|-------------|
| `service_binding` | OData/RAP service binding name |
| `service_definition` | OData/RAP service definition name |

### Other

| Parameter | Description |
|-----------|-------------|
| `notes` | Additional notes to include in the spec |

---

## Example Usage (in Claude)

**Minimal — just a transport number:**
```
GenerateWricefTechSpec(
  transport_numbers = "DEVK900123"
)
```

**Full Report spec:**
```
GenerateWricefTechSpec(
  transport_numbers  = "DEVK900123,DEVK900124",
  wricef_type        = "REPORT",
  wricef_id          = "ZSD-R-001",
  complexity         = "M",
  cr_number          = "CHG0012345",
  module             = "SD",
  developer_name     = "John Smith",
  developer_user_id  = "JSMITH",
  reviewer_name      = "Mary Johnson",
  system_id          = "DEV",
  client             = "100"
)
```

**Enhancement spec:**
```
GenerateWricefTechSpec(
  transport_numbers  = "DEVK900145",
  wricef_type        = "ENHANCEMENT",
  wricef_id          = "ZFI-E-001",
  complexity         = "S",
  module             = "FI",
  enhancement_type   = "BAdI",
  enhancement_name   = "BADI_ACC_DOCUMENT",
  original_program   = "SAPLGL_ACCOUNT_ASSIGNMENT"
)
```

**Interface spec:**
```
GenerateWricefTechSpec(
  transport_numbers      = "DEVK900130",
  wricef_type            = "INTERFACE",
  wricef_id              = "ZMM-I-001",
  complexity             = "L",
  module                 = "MM",
  interface_direction    = "Outbound",
  interface_protocol     = "IDoc",
  interface_frequency    = "Event-driven"
)
```

---

## Generated Sections

All sections are controlled by `tech-spec-config.json`. The default set (in order):

| # | Section | Description |
|---|---------|-------------|
| 1 | Document Information | Title, WRICEF ID, type, complexity, system, transport numbers |
| 2 | Developer Information | Developer, reviewer, approver |
| 3 | Change Request Reference | CR/ticket number, business justification |
| 4 | WRICEF Classification | Category, complexity, estimated effort, module |
| 5 | Transport Request Details | One metadata block per TR (owner, status, target) |
| 6 | Transport Contents | Full object list with type and TR reference |
| 7 | Business Requirement | Editable placeholder for functional description |
| 8 | Scope & Assumptions | In scope / out of scope / assumptions / dependencies |
| 9 | Technical Design Overview | Object list, types, approach summary |
| 10 | Object Structure | Object tree from ADT (if available) |
| 11 | Type Hierarchy | Class hierarchy (if available) |
| 12 | **Object Relationship Diagram** | Mermaid diagram — code objects only, colour-coded by type |
| 13 | Report Details | Selection screen parameters + ALV output layout |
| 14 | Interface Details | Direction, protocol, frequency, source/target systems |
| 15 | Message Structure / Field Mapping | Source → target field mapping table |
| 16 | Enhancement Point Details | Enhancement type, name, original object, activation |
| 17 | Form Details | Technology, output type, form/interface name |
| 18 | Workflow Details | Workflow steps |
| 19 | OData / RAP Details | Service binding and definition names |
| 20 | Processing Logic (Pseudocode) | Extracted pseudocode per source object |
| 21 | Call Graph | Call graph from ADT (if available) |
| 22 | Database Objects Used | Tables / views detected from source |
| 23 | Called Function Modules / BAPIs / Methods | FM calls detected from source |
| 24 | Authorization Objects | Auth objects detected from source |
| 25 | ATC Findings | ATC check results (if available) |
| 26 | Unit Test Results | Unit test results (if available) |
| 27 | Error Handling | Error patterns detected from source |
| 28 | Performance Considerations | Anti-patterns detected (SELECT *, SELECT in LOOP) |
| 29 | Test Scenarios | 5-row editable test case table |
| 30 | Additional Notes | Free-text notes from `notes` parameter |
| 31 | Change History | Version table (initial row pre-filled) |
| 32 | Sign-off | Developer / reviewer / approver sign-off table |

---

## Mermaid Object Relationship Diagram

The diagram is automatically generated from transport objects and — when source code is fetched — enriched with edges for:

- `INCLUDE` statements → `includes` edge
- `CALL FUNCTION` / `CALL METHOD` → `calls FM` / `calls method` edge
- `CREATE OBJECT` / `NEW` → `instantiates` edge

**Code object types displayed** (tables, structures, domains etc. are excluded):

| Type | Colour | Represents |
|------|--------|------------|
| `PROG` | Blue | Report / Program |
| `INCL` | Light Blue | Include |
| `CLAS` | Green | Class |
| `INTF` | Light Green | Interface |
| `FUNC` | Orange | Function Module |
| `FUGR` | Light Orange | Function Group |
| `ENHO` | Red | Enhancement Implementation |
| `ENHS` | Light Red | Enhancement Spot |
| `BADI` | Red-Orange | BAdI Definition |
| `SFPF` | Purple | SmartForms Form |
| `SSFO` | Violet | Adobe Form |
| `SRVB` | Dark Teal | OData Service Binding |
| `SRVD` | Teal | Service Definition |
| `BDEF` | Light Teal | Behaviour Definition |
| `DDLS` | CDS Teal | CDS View (DDL Source) |
| `WDYN` | Indigo | Web Dynpro Application |

When a WRICEF spans **multiple transport requests**, each TR appears as a labelled subgraph.

---

## Configuring Sections — `tech-spec-config.json`

Place `tech-spec-config.json` in the same directory as the binary (or the working directory) to customise which sections appear and in what order:

```json
{
  "sections": [
    "documentInfo",
    "developerInfo",
    "changeRequest",
    "wricefClassification",
    "transportDetails",
    "transportContents",
    "businessRequirement",
    "scopeAssumptions",
    "technicalDesign",
    "objectRelationDiagram",
    "reportDetails",
    "interfaceDetails",
    "messageMapping",
    "enhancementDetails",
    "formDetails",
    "workflowDetails",
    "odataDetails",
    "pseudocode",
    "databaseObjects",
    "calledFunctions",
    "authorizationObjects",
    "errorHandling",
    "performanceConsiderations",
    "testScenarios",
    "additionalNotes",
    "changeHistory",
    "signOff"
  ],
  "sectionTitles": {
    "documentInfo": "Document Information",
    "objectRelationDiagram": "Object Relationship Diagram",
    "reportDetails": "Selection Screen / Input Parameters"
  }
}
```

- **`sections`** — ordered list of section IDs to include; omit any ID to suppress that section
- **`sectionTitles`** — override any heading text; only specify sections whose titles you want to change

The full list of 32 section IDs and their default titles is in the [tech-spec-config.json](tech-spec-config.json) file at the project root.

---

## Sample Output

See [SAMPLE_WRICEF_TECH_SPEC.md](SAMPLE_WRICEF_TECH_SPEC.md) for a complete example showing:
- **R — Report** spec (ZSD-R-001) with selection screen, ALV layout, pseudocode, DB tables, FM calls, ATC, auth objects
- **I — Interface** spec (ZMM-I-001) with field mapping table
- **E — Enhancement** spec (ZFI-E-001) with BAdI details and pseudocode

---

## Configuration Reference

| Environment Variable | CLI Flag | Description |
|---------------------|----------|-------------|
| `SAP_URL` | `--url` | SAP system URL (e.g. `https://host:44300`) |
| `SAP_USER` | `--user` | SAP username |
| `SAP_PASSWORD` | `--password` | SAP password |
| `SAP_CLIENT` | `--client` | SAP client (default: `001`) |
| `SAP_INSECURE` | `--insecure` | Skip TLS verification (for self-signed certs) |
| `SAP_MODE` | `--mode` | `focused` (default) or `expert` (all tools) |

---

## Source Code Structure

```
ABAPDocMCP/
├── cmd/vsp/main.go                   # Entry point (CLI + MCP server)
├── internal/
│   ├── mcp/
│   │   ├── server.go                 # Tool registration
│   │   └── handlers_techspec.go      # GenerateWricefTechSpec handler
│   └── techspec/
│       ├── config.go                 # Section config loader (tech-spec-config.json)
│       ├── mermaid.go                # Mermaid diagram generator
│       └── generator.go             # 32-section spec renderer
├── pkg/adt/                          # SAP ADT REST API client
│   ├── client.go
│   ├── transport.go                  # GetTransport, ListTransports
│   └── workflows.go                  # GetSource (PROG, CLAS, FUNC, INCL …)
├── tech-spec-config.json             # Default section order & titles
└── SAMPLE_WRICEF_TECH_SPEC.md        # Sample generated output
```

---

## Building

```bash
# Current platform
go build -o abapdocmcp ./cmd/vsp/main.go

# All platforms (requires make)
make build-all
```

---

## License

MIT
