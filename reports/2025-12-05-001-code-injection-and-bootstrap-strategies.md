# Code Injection & Bootstrap Strategies via ADT

**Date:** 2025-12-05
**Report ID:** 001
**Subject:** Strategies for code execution and software deployment via ADT without OData services
**Related Documents:** DSL.md, abapGit integration plans

---

## Executive Summary

This report explores creative approaches to:
1. Execute arbitrary ABAP code via ADT without custom OData/REST services
2. Inject data and payloads into SAP systems
3. Bootstrap software deployment (including abapGit) via DSL workflows
4. Create a self-contained deployment pipeline using only ADT capabilities

---

## 1. Code Execution Strategies

### 1.1 Unit Test as Code Execution Vehicle (⭐ Most Promising)

**Concept:** Create a temporary test class, inject code as test method, execute via RunUnitTests, parse results.

```abap
CLASS ltc_executor DEFINITION FOR TESTING RISK LEVEL HARMLESS.
  PUBLIC SECTION.
    METHODS execute_payload FOR TESTING.
ENDCLASS.

CLASS ltc_executor IMPLEMENTATION.
  METHOD execute_payload.
    " === INJECTED PAYLOAD START ===
    DATA(lv_result) = 'Hello from injected code'.
    " Can call any ABAP code here
    cl_abap_unit_assert=>fail( msg = lv_result ). " Return data via assertion message
    " === INJECTED PAYLOAD END ===
  ENDMETHOD.
ENDCLASS.
```

**Workflow:**
```yaml
# execute-code.yaml
name: Execute Arbitrary Code
steps:
  - name: Create executor program
    action: WriteSource
    params:
      type: PROG
      name: ZTEMP_EXECUTOR_$TIMESTAMP
      package: $TMP
      source: |
        REPORT ztemp_executor_$TIMESTAMP.
        CLASS ltc_exec DEFINITION FOR TESTING.
          PUBLIC SECTION.
            METHODS run FOR TESTING.
        ENDCLASS.
        CLASS ltc_exec IMPLEMENTATION.
          METHOD run.
            $PAYLOAD
            cl_abap_unit_assert=>fail( msg = |result:{ lv_result }| ).
          ENDMETHOD.
        ENDCLASS.

  - name: Run and capture output
    action: RunUnitTests
    params:
      object_url: /sap/bc/adt/programs/programs/ZTEMP_EXECUTOR_$TIMESTAMP

  - name: Cleanup
    action: DeleteObject
    params:
      object_url: /sap/bc/adt/programs/programs/ZTEMP_EXECUTOR_$TIMESTAMP
```

**Pros:**
- Works with any ADT-enabled system
- No special permissions beyond developer access
- Can return structured data via assertion messages
- Supports RISK LEVEL DANGEROUS for DB operations

**Cons:**
- Output size limited by assertion message length
- Requires $TMP or transport for non-local
- Cleanup needed

**Data Return Strategies:**
1. **Assertion messages** - Up to ~1000 chars per message
2. **Multiple assertions** - Chain results across multiple fail() calls
3. **Write to table** - Store results in Z-table, read via GetTableContents
4. **MIME object** - Write results to SMW0, read back

---

### 1.2 Include-Based Payload Injection

**Concept:** Create an include with payload, reference from main program.

```yaml
# inject-include.yaml
steps:
  - name: Create payload include
    action: WriteSource
    params:
      type: INCL
      name: ZINCL_PAYLOAD_001
      source: |
        " Auto-generated payload
        DATA: gt_result TYPE string_table.
        APPEND 'Line 1' TO gt_result.
        APPEND 'Line 2' TO gt_result.

  - name: Create executor with include
    action: WriteSource
    params:
      type: PROG
      name: ZEXEC_WITH_INCLUDE
      source: |
        REPORT zexec_with_include.
        INCLUDE zincl_payload_001.
        " Rest of program uses gt_result
```

**Use Case:** Inject large data sets or complex logic as reusable includes.

---

### 1.3 Function Module Execution (If Available)

Some systems expose RFC-enabled function modules via ADT:

```
POST /sap/bc/adt/functions/{fugr}/{func}/executions
```

**Known executable FMs:**
- `RFC_READ_TABLE` - Read any table
- `BAPI_*` - Business APIs
- Custom Z* RFC-enabled FMs

**Limitation:** Requires FM to be RFC-enabled.

---

## 2. Data Injection Strategies

### 2.1 SQL Query to Z-Tables

**Concept:** Use RunQuery to INSERT data into custom tables.

```sql
-- Create staging table first (via DDIC or code)
INSERT INTO ztmp_payload VALUES ('KEY1', 'PAYLOAD_DATA_BASE64...')
```

**Workflow:**
```yaml
steps:
  - name: Inject payload data
    action: RunQuery
    params:
      sql_query: |
        INSERT INTO ztmp_staging
        SELECT 'PKG1' AS package, 'ZCL_MY_CLASS' AS name,
               '$BASE64_SOURCE' AS source
        FROM t000 WHERE mandt = '000'
```

**Limitation:** INSERT requires specific DB permissions, may not work on all systems.

---

### 2.2 MIME Repository Objects (W3MI/SMW0) - ⚠️ NOT SUPPORTED

**Status:** MIME objects are **NOT directly accessible** via ADT REST API.

**Workaround Options:**

1. **Via Unit Test Execution** - Inject code that creates MIME objects:
```abap
" Inside test method - creates MIME object programmatically
DATA(lo_mime) = cl_mime_repository=>get_api( ).
lo_mime->put(
  EXPORTING
    i_url = '/SAP/PUBLIC/ZPAYLOAD_001'
    i_content = lv_binary_content
).
```

2. **Via Custom Table** - Store binary data as RAWSTRING:
```sql
INSERT INTO ztab_binary_store VALUES (
  'PAYLOAD_001',
  $BASE64_DECODED_CONTENT
)
```

3. **Via BSP Application** (if BSP Workbench available):
   - Create BSP app via ADT
   - Upload files to BSP MIME repository

**Conclusion:** For binary storage, prefer **Table + RAWSTRING** or **Include + BASE64** approaches.

---

### 2.3 Message Class as Data Store

**Concept:** Abuse message class texts as key-value store.

```yaml
steps:
  - name: Create message class with data
    action: WriteSource
    params:
      type: MSAG
      name: ZDATA_STORE
      # Messages as KV pairs:
      # 001 = {"key": "value1"}
      # 002 = {"key": "value2"}
```

**ABAP Read:**
```abap
MESSAGE e001(zdata_store) INTO DATA(lv_json).
```

**Limitation:** ~255 chars per message, but unlimited messages.

---

### 2.4 Domain Fixed Values

**Concept:** Store configuration in domain fixed values.

```
Domain: ZCONFIG_VALUES
Fixed Values:
  'KEY1' = 'Value 1'
  'KEY2' = 'Value 2'
```

**Use Case:** Configuration that survives transport.

---

## 3. Bootstrap Strategies

### 3.1 Minimal Bootstrap Package

**Goal:** Deploy minimum code needed to enable full deployment pipeline.

```
ZBOOTSTRAP/
├── ZCL_BOOTSTRAP_LOADER     # Main loader class
├── ZINCL_BOOTSTRAP_CONFIG   # Configuration include
└── ZTAB_BOOTSTRAP_QUEUE     # Deployment queue table
```

**Bootstrap Workflow:**
```yaml
# bootstrap.yaml - Run once to enable full deployment
name: Bootstrap Deployment Infrastructure
variables:
  PACKAGE: $TMP  # Or ZBOOTSTRAP for persistent

steps:
  # Step 1: Create config table
  - name: Create deployment queue table
    action: CreateObject
    params:
      type: TABL
      name: ZTAB_DEPLOY_QUEUE
      # ... table definition

  # Step 2: Create loader class
  - name: Create bootstrap loader
    action: WriteSource
    params:
      type: CLAS
      name: ZCL_BOOTSTRAP
      source: |
        CLASS zcl_bootstrap DEFINITION PUBLIC.
          PUBLIC SECTION.
            CLASS-METHODS:
              load_from_url IMPORTING iv_url TYPE string,
              load_from_table,
              process_queue.
        ENDCLASS.
        " ... implementation
```

---

### 3.2 abapGit Bootstrap via DSL

**Goal:** Install abapGit using only ADT operations.

**Challenge:** abapGit is ~50+ objects, complex dependencies.

**Strategy 1: Standalone File**
```yaml
# install-abapgit.yaml
name: Install abapGit Standalone
steps:
  - name: Download abapGit standalone
    action: DownloadFile  # External step
    params:
      url: https://raw.githubusercontent.com/abapGit/build/main/zabapgit_standalone.prog.abap
      output: /tmp/zabapgit.abap

  - name: Deploy to SAP
    action: ImportFromFile
    params:
      file_path: /tmp/zabapgit.abap
      package: $TMP
```

**Strategy 2: Chunked Deployment**
```yaml
# For full abapGit (multi-object)
name: Install abapGit Full
steps:
  - name: Create package
    action: CreateObject
    params:
      type: DEVC
      name: $ABAPGIT

  - name: Deploy objects in order
    action: BatchDeploy
    params:
      manifest: abapgit-objects.json
      # Objects in dependency order
```

---

### 3.3 Self-Replicating Deployment Agent

**Concept:** Deploy minimal agent that can pull and deploy more code.

```abap
CLASS zcl_deploy_agent DEFINITION PUBLIC.
  PUBLIC SECTION.
    METHODS:
      " Pull source from external URL
      pull_source
        IMPORTING iv_url TYPE string
        RETURNING VALUE(rv_source) TYPE string,

      " Deploy source to system
      deploy
        IMPORTING iv_name TYPE string
                  iv_source TYPE string
                  iv_type TYPE string,

      " Process deployment manifest
      process_manifest
        IMPORTING iv_manifest_url TYPE string.
ENDCLASS.
```

**Usage via Unit Test execution:**
```yaml
steps:
  - name: Deploy agent
    action: WriteSource
    params:
      type: CLAS
      name: ZCL_DEPLOY_AGENT
      source: $AGENT_SOURCE

  - name: Execute deployment
    action: ExecuteViaUnitTest
    params:
      class: ZCL_DEPLOY_AGENT
      payload: |
        NEW zcl_deploy_agent( )->process_manifest(
          'https://myrepo.com/manifest.json'
        ).
```

---

## 4. DSL Extensions for Deployment

### 4.1 Proposed DSL Actions

```yaml
# New actions for deployment workflows
actions:
  # Execute arbitrary ABAP via unit test wrapper
  ExecuteABAP:
    params:
      code: string        # ABAP code to execute
      return_var: string  # Variable name to return
      risk_level: harmless|dangerous|critical

  # Inject data into system
  InjectData:
    params:
      target: table|mime|message
      name: string
      data: string|base64

  # Conditional deployment
  DeployIfNotExists:
    params:
      object_type: string
      object_name: string
      source: string

  # Bootstrap sequence
  Bootstrap:
    params:
      manifest_url: string
      target_package: string
```

### 4.2 Deployment Manifest Format

```json
{
  "name": "My Application",
  "version": "1.0.0",
  "target_package": "ZMYAPP",
  "dependencies": [
    {"type": "abapgit", "url": "https://github.com/org/repo.git"}
  ],
  "objects": [
    {
      "type": "CLAS",
      "name": "ZCL_MYAPP_MAIN",
      "source_url": "https://raw.githubusercontent.com/.../zcl_myapp_main.clas.abap",
      "order": 1
    },
    {
      "type": "TABL",
      "name": "ZTAB_CONFIG",
      "source_url": "...",
      "order": 0
    }
  ],
  "post_install": [
    {"action": "ExecuteABAP", "code": "zcl_myapp_main=>initialize( )."}
  ]
}
```

---

## 5. Security Considerations

### 5.1 Risks

| Risk | Mitigation |
|------|------------|
| Arbitrary code execution | Require explicit `--enable-code-exec` flag |
| Data exfiltration via assertions | Limit assertion message parsing |
| Persistent backdoors | Cleanup temp objects, audit trail |
| Transport pollution | Use $TMP by default, warn on transport |

### 5.2 Audit Trail

```yaml
# All deployment operations should be logged
audit:
  log_level: INFO
  log_target:
    - stderr
    - sap_table: ZTAB_DEPLOY_LOG
  fields:
    - timestamp
    - user
    - action
    - object
    - status
```

---

## 6. Implementation Roadmap

### Phase 1: Core Execution (Effort: 3)
- [ ] `ExecuteABAP` action via unit test wrapper
- [ ] Result parsing from assertion messages
- [ ] Automatic cleanup of temp objects

### Phase 2: Data Injection (Effort: 4)
- [ ] MIME object support (W3MI)
- [ ] Table data injection (where permitted)
- [ ] Large payload chunking

### Phase 3: Bootstrap Framework (Effort: 5)
- [ ] Deployment manifest format
- [ ] Dependency resolution
- [ ] abapGit standalone installer

### Phase 4: abapGit Integration (Effort: 6)
- [ ] Optional abapGit REST API wrapper
- [ ] Repo link/unlink
- [ ] Pull/push operations
- [ ] Conflict resolution UI

---

## 7. Proof of Concept Ideas

### 7.1 "Hello World" Code Execution

```yaml
# poc-execute.yaml
name: Execute Hello World
steps:
  - action: ExecuteABAP
    params:
      code: |
        DATA(lv_msg) = |Hello from SAP at { sy-datum } { sy-uzeit }|.
        DATA(lv_user) = sy-uname.
      return_var: lv_msg
```

### 7.2 System Info Extraction

```yaml
# poc-sysinfo.yaml
name: Get System Information
steps:
  - action: ExecuteABAP
    params:
      code: |
        DATA: lv_result TYPE string.
        SELECT SINGLE * FROM t000 INTO @DATA(ls_client).
        lv_result = |Client: { ls_client-mandt }, System: { sy-sysid }|.
      return_var: lv_result
```

### 7.3 Install Library from GitHub

```yaml
# poc-install-lib.yaml
name: Install JSON Library
steps:
  - action: DeployFromURL
    params:
      url: https://raw.githubusercontent.com/user/repo/main/zcl_json.clas.abap
      package: $TMP
      activate: true
```

---

## 8. Conclusion

The combination of:
1. **Unit Test execution** as code runner
2. **MIME/Table storage** for data injection
3. **DSL workflows** for orchestration

Creates a powerful deployment pipeline that works on any ADT-enabled system without requiring:
- Custom OData services
- RFC destinations
- Special authorizations beyond developer access

**Next Steps:**
1. Implement `ExecuteABAP` action POC
2. Test MIME object create/read via ADT
3. Design deployment manifest schema
4. Create abapGit standalone installer workflow

---

## Appendix: ADT Object Types for Injection

| Object Type | ADT Support | Data Capacity | Persistence | Notes |
|-------------|-------------|---------------|-------------|-------|
| Program (PROG) | ✅ Full CRUD | Unlimited | Permanent | Best for code injection |
| Class (CLAS) | ✅ Full CRUD | Unlimited | Permanent | Best for code injection |
| Include (INCL) | ✅ Full CRUD | Unlimited | Permanent | Good for data chunks |
| Table (TABL) | ✅ Structure only | N/A | Permanent | Schema only |
| Table Data | ⚠️ Via RunQuery | Row limits | Permanent | SELECT works, INSERT needs perms |
| MIME (W3MI) | ❌ Not via ADT | ~10MB | Permanent | Use Unit Test workaround |
| Message (MSAG) | ✅ Read via GetSource | 255 chars/msg | Permanent | Write needs research |
| Domain | ❌ Not via ADT | ~60 chars/value | Permanent | DDIC only |
| Function Module | ✅ Full CRUD | Unlimited | Permanent | Good for callable code |
| Interface | ✅ Full CRUD | Unlimited | Permanent | Type definitions |

