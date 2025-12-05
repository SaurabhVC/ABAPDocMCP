# ADT-Assisted Universal Deployment Strategy

**Date:** 2025-12-05
**Report ID:** 003
**Subject:** Deploy ANY SAP object type via ADT + Factory Pattern
**Related:** 2025-12-05-001, 2025-12-05-002

---

## 1. Core Insight

```
┌─────────────────────────────────────────────────────────────────────┐
│  vsp (your machine)                                                  │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  - Full internet access                                      │    │
│  │  - Can download from GitHub, npm, anywhere                   │    │
│  │  - Prepares payloads locally                                 │    │
│  │  - Pushes via ADT (reliable, authenticated)                  │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              │ ADT REST API (HTTP/HTTPS)
                              │ (SAP trusts this connection)
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  SAP System                                                          │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  Direct ADT support:                                         │    │
│  │  ✅ PROG, CLAS, INTF, INCL, FUNC, FUGR, DDLS                │    │
│  │                                                              │    │
│  │  NOT directly via ADT:                                       │    │
│  │  ❌ TABL, DTEL, DOMA, TTYP, VIEW, SHLP, ENQU, TRAN, etc.   │    │
│  │                                                              │    │
│  │  SOLUTION: Factory Program + Unit Test Execution             │    │
│  │  ✅ ANY object via ABAP APIs (RS_*, DDIF_*, etc.)           │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Two-Tier Deployment Architecture

### Tier 1: Direct ADT Objects (Native)

| Object Type | ADT Endpoint | vsp Tool |
|-------------|--------------|----------|
| PROG | `/sap/bc/adt/programs/programs/{name}` | WriteSource |
| CLAS | `/sap/bc/adt/oo/classes/{name}` | WriteSource |
| INTF | `/sap/bc/adt/oo/interfaces/{name}` | WriteSource |
| INCL | `/sap/bc/adt/programs/includes/{name}` | WriteSource |
| FUGR | `/sap/bc/adt/functions/groups/{name}` | WriteSource |
| FUNC | `/sap/bc/adt/functions/groups/{fugr}/fmodules/{name}` | WriteSource |
| DDLS | `/sap/bc/adt/ddic/ddl/sources/{name}` | WriteSource |

### Tier 2: Factory-Created Objects (via Code Execution)

| Object Type | ABAP API | Use Case |
|-------------|----------|----------|
| TABL | `DDIF_TABL_PUT` | Tables |
| DTEL | `DDIF_DTEL_PUT` | Data Elements |
| DOMA | `DDIF_DOMA_PUT` | Domains |
| TTYP | `DDIF_TTYP_PUT` | Table Types |
| VIEW | `DDIF_VIEW_PUT` | Database Views |
| SHLP | `DDIF_SHLP_PUT` | Search Helps |
| ENQU | `DDIF_ENQU_PUT` | Lock Objects |
| TRAN | `SMEN_TRANSACTION_*` | Transactions |
| MSAG | `RS_MESSAGE_*` | Messages |
| AUTH | `PRGN_*` | Authorizations |

---

## 3. Factory Program Pattern

### 3.1 Architecture

```
┌────────────────────────────────────────────────────────────────┐
│  ZFACTORY_DEPLOY_$TIMESTAMP                                     │
│  ├── INCLUDE zfactory_payload   (BASE64 zipped object defs)    │
│  ├── CLASS lcl_factory          (unpacker + creator logic)     │
│  └── CLASS ltc_executor         (unit test trigger)            │
└────────────────────────────────────────────────────────────────┘
                              │
                              │ RunUnitTests
                              ▼
┌────────────────────────────────────────────────────────────────┐
│  Execution Flow:                                                │
│  1. Decode BASE64 payload                                       │
│  2. Unzip to get object definitions (JSON/XML)                  │
│  3. For each object: call appropriate DDIF_*/RS_* API           │
│  4. Activate objects                                            │
│  5. Return results via assertion message                        │
└────────────────────────────────────────────────────────────────┘
```

### 3.2 Factory Program Template

```abap
REPORT zfactory_deploy_$GUID.

*----------------------------------------------------------------------*
* PAYLOAD INCLUDE - Contains BASE64 encoded, zipped object definitions
*----------------------------------------------------------------------*
INCLUDE zfactory_payload_$GUID.
" Defines: gc_payload TYPE string VALUE '...'

*----------------------------------------------------------------------*
* FACTORY CLASS - Unpacks and creates objects
*----------------------------------------------------------------------*
CLASS lcl_object_factory DEFINITION.
  PUBLIC SECTION.
    TYPES:
      BEGIN OF ty_object_def,
        obj_type    TYPE string,    " TABL, DTEL, DOMA, etc.
        obj_name    TYPE string,
        package     TYPE devclass,
        definition  TYPE string,    " JSON or XML definition
      END OF ty_object_def,
      tt_objects TYPE STANDARD TABLE OF ty_object_def.

    TYPES:
      BEGIN OF ty_result,
        obj_name TYPE string,
        status   TYPE string,
        message  TYPE string,
      END OF ty_result,
      tt_results TYPE STANDARD TABLE OF ty_result.

    CLASS-METHODS:
      execute
        RETURNING VALUE(rt_results) TYPE tt_results,

      decode_payload
        RETURNING VALUE(rt_objects) TYPE tt_objects,

      create_object
        IMPORTING is_object TYPE ty_object_def
        RETURNING VALUE(rs_result) TYPE ty_result.

  PRIVATE SECTION.
    CLASS-METHODS:
      create_table IMPORTING is_def TYPE ty_object_def RETURNING VALUE(rv_ok) TYPE abap_bool,
      create_data_element IMPORTING is_def TYPE ty_object_def RETURNING VALUE(rv_ok) TYPE abap_bool,
      create_domain IMPORTING is_def TYPE ty_object_def RETURNING VALUE(rv_ok) TYPE abap_bool,
      create_table_type IMPORTING is_def TYPE ty_object_def RETURNING VALUE(rv_ok) TYPE abap_bool,
      create_message_class IMPORTING is_def TYPE ty_object_def RETURNING VALUE(rv_ok) TYPE abap_bool,
      activate_objects IMPORTING it_objects TYPE tt_objects.
ENDCLASS.

CLASS lcl_object_factory IMPLEMENTATION.

  METHOD execute.
    " 1. Decode payload
    DATA(lt_objects) = decode_payload( ).

    " 2. Create each object
    LOOP AT lt_objects INTO DATA(ls_object).
      APPEND create_object( ls_object ) TO rt_results.
    ENDLOOP.

    " 3. Mass activation
    activate_objects( lt_objects ).
  ENDMETHOD.

  METHOD decode_payload.
    " Decode BASE64
    DATA(lv_zipped) = cl_http_utility=>decode_base64( gc_payload ).

    " Unzip
    DATA(lo_zip) = NEW cl_abap_zip( ).
    lo_zip->load( lv_zipped ).

    " Extract manifest.json
    DATA lv_manifest TYPE xstring.
    lo_zip->get( EXPORTING name = 'manifest.json' IMPORTING content = lv_manifest ).

    " Convert to string and parse
    DATA(lv_json) = cl_abap_conv_codepage=>create_in( )->convert( lv_manifest ).

    " Parse JSON to object list
    " (Using /ui2/cl_json or simple parsing)
    /ui2/cl_json=>deserialize( EXPORTING json = lv_json CHANGING data = rt_objects ).
  ENDMETHOD.

  METHOD create_object.
    rs_result-obj_name = is_object-obj_name.

    TRY.
        CASE is_object-obj_type.
          WHEN 'TABL'. rs_result-status = COND #( WHEN create_table( is_object ) THEN 'OK' ELSE 'FAIL' ).
          WHEN 'DTEL'. rs_result-status = COND #( WHEN create_data_element( is_object ) THEN 'OK' ELSE 'FAIL' ).
          WHEN 'DOMA'. rs_result-status = COND #( WHEN create_domain( is_object ) THEN 'OK' ELSE 'FAIL' ).
          WHEN 'TTYP'. rs_result-status = COND #( WHEN create_table_type( is_object ) THEN 'OK' ELSE 'FAIL' ).
          WHEN 'MSAG'. rs_result-status = COND #( WHEN create_message_class( is_object ) THEN 'OK' ELSE 'FAIL' ).
          WHEN OTHERS. rs_result-status = 'UNSUPPORTED'.
        ENDCASE.
      CATCH cx_root INTO DATA(lx_error).
        rs_result-status = 'ERROR'.
        rs_result-message = lx_error->get_text( ).
    ENDTRY.
  ENDMETHOD.

  METHOD create_table.
    " Parse table definition from JSON
    DATA: ls_dd02v TYPE dd02v,
          lt_dd03p TYPE STANDARD TABLE OF dd03p.

    " ... parse is_def-definition into ls_dd02v, lt_dd03p ...

    " Create table
    CALL FUNCTION 'DDIF_TABL_PUT'
      EXPORTING
        name      = CONV ddobjname( is_def-obj_name )
        dd02v_wa  = ls_dd02v
      TABLES
        dd03p_tab = lt_dd03p
      EXCEPTIONS
        OTHERS    = 1.

    rv_ok = xsdbool( sy-subrc = 0 ).
  ENDMETHOD.

  METHOD create_data_element.
    DATA: ls_dd04v TYPE dd04v.
    " ... parse and call DDIF_DTEL_PUT ...
    rv_ok = abap_true.
  ENDMETHOD.

  METHOD create_domain.
    DATA: ls_dd01v TYPE dd01v.
    " ... parse and call DDIF_DOMA_PUT ...
    rv_ok = abap_true.
  ENDMETHOD.

  METHOD create_table_type.
    DATA: ls_dd40v TYPE dd40v.
    " ... parse and call DDIF_TTYP_PUT ...
    rv_ok = abap_true.
  ENDMETHOD.

  METHOD create_message_class.
    " ... call RS_MESSAGE_* or direct table insert ...
    rv_ok = abap_true.
  ENDMETHOD.

  METHOD activate_objects.
    DATA: lt_objects TYPE STANDARD TABLE OF dwinactiv.

    LOOP AT it_objects INTO DATA(ls_obj).
      APPEND VALUE #(
        object   = CONV #( ls_obj-obj_type )
        obj_name = CONV #( ls_obj-obj_name )
      ) TO lt_objects.
    ENDLOOP.

    CALL FUNCTION 'RS_WORKING_OBJECTS_ACTIVATE'
      EXPORTING
        activate_ddic_objects = abap_true
      TABLES
        objects               = lt_objects
      EXCEPTIONS
        OTHERS                = 1.
  ENDMETHOD.

ENDCLASS.

*----------------------------------------------------------------------*
* UNIT TEST TRIGGER - Executes factory when tests run
*----------------------------------------------------------------------*
CLASS ltc_executor DEFINITION FOR TESTING
  RISK LEVEL CRITICAL
  DURATION LONG.

  PRIVATE SECTION.
    METHODS execute_deployment FOR TESTING.
ENDCLASS.

CLASS ltc_executor IMPLEMENTATION.
  METHOD execute_deployment.
    " Execute factory
    DATA(lt_results) = lcl_object_factory=>execute( ).

    " Return results via assertion
    DATA(lv_json) = /ui2/cl_json=>serialize( lt_results ).

    " This "fails" the test but returns our data
    cl_abap_unit_assert=>fail( msg = lv_json ).
  ENDMETHOD.
ENDCLASS.
```

---

## 4. vsp DSL Workflow

### 4.1 High-Level Workflow

```yaml
# deploy-ddic-objects.yaml
name: Deploy DDIC Objects via Factory
description: Creates tables, data elements, domains via factory program

variables:
  PACKAGE: $TMP
  TIMESTAMP: $AUTO

steps:
  # Step 1: Prepare payload locally (vsp side)
  - name: Prepare payload
    action: LocalPrepare
    params:
      input_dir: ./ddic-objects/
      output: /tmp/payload.zip
      # Creates ZIP with:
      # - manifest.json (object list)
      # - definitions/*.json (each object definition)

  # Step 2: Create payload include
  - name: Create payload include
    action: WriteSource
    params:
      type: INCL
      name: ZFACTORY_PAYLOAD_$TIMESTAMP
      package: $PACKAGE
      source: |
        CONSTANTS: gc_payload TYPE string VALUE
          '$BASE64_CONTENT' ##NO_TEXT.

  # Step 3: Create factory program
  - name: Create factory program
    action: WriteSource
    params:
      type: PROG
      name: ZFACTORY_DEPLOY_$TIMESTAMP
      package: $PACKAGE
      source_template: factory_template.abap
      # Injects: INCLUDE zfactory_payload_$TIMESTAMP.

  # Step 4: Execute via unit test
  - name: Execute factory
    action: RunUnitTests
    params:
      object_url: /sap/bc/adt/programs/programs/ZFACTORY_DEPLOY_$TIMESTAMP
      include_dangerous: true
      # Parse results from assertion message

  # Step 5: Cleanup factory (optional)
  - name: Cleanup
    action: DeleteObject
    params:
      object_url: /sap/bc/adt/programs/programs/ZFACTORY_DEPLOY_$TIMESTAMP
    when: cleanup_enabled
```

### 4.2 Object Definition Format

```json
{
  "objects": [
    {
      "type": "DOMA",
      "name": "ZDOMA_STATUS",
      "package": "$TMP",
      "definition": {
        "datatype": "CHAR",
        "length": 10,
        "description": "Status Domain",
        "fixed_values": [
          {"low": "NEW", "description": "New"},
          {"low": "PROC", "description": "Processing"},
          {"low": "DONE", "description": "Done"}
        ]
      }
    },
    {
      "type": "DTEL",
      "name": "ZDTEL_STATUS",
      "package": "$TMP",
      "definition": {
        "domain": "ZDOMA_STATUS",
        "description": "Status Data Element",
        "labels": {
          "short": "Status",
          "medium": "Status",
          "long": "Record Status",
          "heading": "Status"
        }
      }
    },
    {
      "type": "TABL",
      "name": "ZTAB_ORDERS",
      "package": "$TMP",
      "definition": {
        "table_type": "TRANSP",
        "description": "Orders Table",
        "fields": [
          {"name": "MANDT", "type": "MANDT", "key": true},
          {"name": "ORDER_ID", "type": "CHAR", "length": 20, "key": true},
          {"name": "STATUS", "type": "ZDTEL_STATUS"},
          {"name": "CREATED", "type": "TIMESTAMPL"}
        ]
      }
    }
  ]
}
```

---

## 5. Supported Object Types via Factory

### 5.1 DDIC Objects

| Type | ABAP API | Complexity | Status |
|------|----------|------------|--------|
| DOMA | `DDIF_DOMA_PUT` | Low | ✅ Ready |
| DTEL | `DDIF_DTEL_PUT` | Low | ✅ Ready |
| TABL | `DDIF_TABL_PUT` | Medium | ✅ Ready |
| TTYP | `DDIF_TTYP_PUT` | Low | ✅ Ready |
| VIEW | `DDIF_VIEW_PUT` | Medium | ✅ Ready |
| SHLP | `DDIF_SHLP_PUT` | Medium | ✅ Ready |
| ENQU | `DDIF_ENQU_PUT` | Low | ✅ Ready |
| STRU | `DDIF_TABL_PUT` (type=INTTAB) | Low | ✅ Ready |

### 5.2 Other Objects

| Type | ABAP API | Complexity | Status |
|------|----------|------------|--------|
| MSAG | `RS_MESSAGE_INSERT` | Low | ✅ Ready |
| TRAN | Direct TSTC insert + `SMEN_BUFFER_REFRESH` | Medium | 🔄 Needs test |
| AUTH | `PRGN_*` FMs | High | 📋 Planned |
| NROB | `NUMBER_RANGE_*` | Medium | 📋 Planned |

### 5.3 Complex Objects (via serialization)

| Type | Approach | Complexity |
|------|----------|------------|
| WDYN | Export/Import via `CL_WDY_*` | Very High |
| SICF | Direct table insert | Medium |
| SPRX | Web Service config | High |

---

## 6. Result Parsing

### 6.1 Unit Test Output Format

```json
// Returned via cl_abap_unit_assert=>fail( msg = ... )
{
  "status": "completed",
  "objects_created": 3,
  "objects_failed": 0,
  "results": [
    {"name": "ZDOMA_STATUS", "type": "DOMA", "status": "OK"},
    {"name": "ZDTEL_STATUS", "type": "DTEL", "status": "OK"},
    {"name": "ZTAB_ORDERS", "type": "TABL", "status": "OK"}
  ],
  "activation": {
    "status": "OK",
    "warnings": []
  }
}
```

### 6.2 vsp Result Parser

```go
// Parse unit test results for factory output
func parseFactoryResult(unitTestResult *adt.UnitTestResult) (*FactoryResult, error) {
    // Find the assertion message containing our JSON
    for _, testClass := range unitTestResult.TestClasses {
        for _, method := range testClass.Methods {
            for _, alert := range method.Alerts {
                if strings.HasPrefix(alert.Message, "{") {
                    var result FactoryResult
                    if err := json.Unmarshal([]byte(alert.Message), &result); err == nil {
                        return &result, nil
                    }
                }
            }
        }
    }
    return nil, errors.New("factory result not found in unit test output")
}
```

---

## 7. Chunking Large Payloads

### 7.1 Size Limits

| Constraint | Limit | Solution |
|------------|-------|----------|
| String literal in ABAP | ~1 MB practical | Multiple includes |
| Program size | ~16 MB | OK for most payloads |
| Unit test timeout | System dependent | Split into batches |

### 7.2 Multi-Include Pattern

```abap
" Main payload assembler
INCLUDE zfactory_payload_001.  " gc_chunk_001
INCLUDE zfactory_payload_002.  " gc_chunk_002
INCLUDE zfactory_payload_003.  " gc_chunk_003

CONSTANTS: gc_payload TYPE string VALUE gc_chunk_001 && gc_chunk_002 && gc_chunk_003.
```

### 7.3 vsp Chunking Logic

```go
func chunkPayload(content []byte, chunkSize int) []string {
    encoded := base64.StdEncoding.EncodeToString(content)
    var chunks []string

    for i := 0; i < len(encoded); i += chunkSize {
        end := i + chunkSize
        if end > len(encoded) {
            end = len(encoded)
        }
        chunks = append(chunks, encoded[i:end])
    }

    return chunks
}
```

---

## 8. Security & Cleanup

### 8.1 Factory Cleanup Options

```yaml
cleanup:
  # Option 1: Delete immediately after execution
  immediate: true

  # Option 2: Keep for debugging, delete later
  keep_hours: 24

  # Option 3: Keep factory, delete only payload includes
  keep_factory: true
  delete_payloads: true
```

### 8.2 Audit Trail

```abap
" Factory logs all creations
DATA: ls_log TYPE ztab_factory_log.
ls_log-timestamp = utclong_current( ).
ls_log-user      = sy-uname.
ls_log-factory   = sy-repid.
ls_log-obj_type  = is_object-obj_type.
ls_log-obj_name  = is_object-obj_name.
ls_log-status    = lv_status.
INSERT ztab_factory_log FROM ls_log.
```

---

## 9. Implementation Plan

| Phase | Effort | Deliverable |
|-------|--------|-------------|
| **Phase 1** | 2 days | Factory template for DOMA/DTEL/TABL |
| **Phase 2** | 1 day | vsp LocalPrepare action (ZIP + BASE64) |
| **Phase 3** | 1 day | vsp workflow + result parsing |
| **Phase 4** | 2 days | Additional object types (TTYP, VIEW, SHLP) |
| **Phase 5** | 1 day | Chunking + large payload support |

**Total: ~7 days for full implementation**

---

## 10. Advantages over Deploy Agent

| Aspect | Deploy Agent | ADT Factory |
|--------|--------------|-------------|
| **Network** | Needs SAP outbound | Only ADT inbound ✅ |
| **SSL/Certs** | STRUST config needed | vsp handles it ✅ |
| **Reliability** | Depends on SAP HTTP client | Fully controlled ✅ |
| **Object Types** | Limited by ADT | ANY via ABAP APIs ✅ |
| **Debugging** | Hard (runs in SAP) | Easy (vsp logs) ✅ |
| **Cleanup** | Manual | Automated ✅ |

---

## 11. Example: Deploy Complete Application

```yaml
# deploy-myapp.yaml
name: Deploy MyApp
description: Full application with DDIC + code

steps:
  # Tier 1: DDIC objects via factory
  - name: Deploy DDIC
    action: FactoryDeploy
    params:
      objects:
        - type: DOMA
          name: ZMYAPP_STATUS
          definition: { datatype: CHAR, length: 10 }
        - type: DTEL
          name: ZMYAPP_STATUS
          definition: { domain: ZMYAPP_STATUS }
        - type: TABL
          name: ZMYAPP_CONFIG
          definition: { fields: [...] }
      package: ZMYAPP

  # Tier 2: Code objects via direct ADT
  - name: Deploy Interface
    action: WriteSource
    params:
      type: INTF
      name: ZIF_MYAPP_SERVICE
      source_file: ./src/zif_myapp_service.intf.abap
      package: ZMYAPP

  - name: Deploy Class
    action: WriteSource
    params:
      type: CLAS
      name: ZCL_MYAPP_SERVICE
      source_file: ./src/zcl_myapp_service.clas.abap
      package: ZMYAPP

  # Post-install
  - name: Run self-test
    action: RunUnitTests
    params:
      object_url: /sap/bc/adt/oo/classes/ZCL_MYAPP_SERVICE
```

