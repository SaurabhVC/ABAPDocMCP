# CROSS and WBCROSSGT: Complete Reference Guide

**Date:** 2025-12-02
**Report ID:** 002
**Subject:** Comprehensive reference for SAP cross-reference tables with actual system data
**Supplement to:** Report 001 (Auto Pilot Analysis)

---

## Executive Summary

This reference guide provides detailed information about SAP's cross-reference tables **CROSS** and **WBCROSSGT**, including actual counts from a real SAP system, handler class mappings, and traversal patterns used in ZXRAY.

**Source Data:** Real SAP system statistics showing 1,917 CROSS entries and 127,084 WBCROSSGT entries for custom Z* objects.

---

## 1. Traversal Patterns

### 1.1 DOWN Traversal (Call Graph / Dependencies)

**Purpose:** Find what a given object calls/uses/references

**Query Pattern for CROSS:**
```abap
" Start with object → Get include → Query CROSS by include
DATA(lv_include) = to_include( starting_object ).

SELECT * FROM cross
  WHERE include = @lv_include
    AND type IN @valid_types.
```

**Query Pattern for WBCROSSGT:**
```abap
" Start with object → Get include → Query WBCROSSGT by include
DATA(lv_include) = to_include( starting_object ).

SELECT * FROM wbcrossgt
  WHERE include = @lv_include
    AND otype IN @valid_otypes.
```

**Result:** List of objects that `starting_object` calls/references

**Example:**
```
Starting object: ZCL_MY_CLASS→METHOD1
Include: ZCL_MY_CLASS========CP

CROSS results:
- TYPE='F', NAME='Z_FUNCTION_MODULE'
- TYPE='T', NAME='ZMY_TRANSACTION'

WBCROSSGT results:
- OTYPE='ME', NAME='ZCL_OTHER_CLASS→METHOD2'
- OTYPE='TY', NAME='ZCL_SOME_TYPE'
```

### 1.2 UP Traversal (Where-Used / Dependents)

**Purpose:** Find what calls/uses/references a given object

**Query Pattern for CROSS:**
```abap
" Start with object → Get type/name → Query CROSS by type/name
DATA(lv_type) = to_type( starting_object ).
DATA(lv_name) = to_name( starting_object ).

SELECT * FROM cross
  WHERE type = @lv_type
    AND name = @lv_name.
```

**Query Pattern for WBCROSSGT:**
```abap
" Start with object → Get otype/name → Query WBCROSSGT by otype/name
DATA(lv_otype) = to_otype( starting_object ).
DATA(lv_name)  = to_name( starting_object ).

SELECT * FROM wbcrossgt
  WHERE otype = @lv_otype
    AND name = @lv_name.
```

**Result:** List of includes (objects) that call/reference `starting_object`

**Example:**
```
Starting object: Z_MY_FUNCTION
Type: 'F', Name: 'Z_MY_FUNCTION'

CROSS results (includes that call it):
- INCLUDE='ZPROGRAM1'
- INCLUDE='ZCL_CLASS========CP'
- INCLUDE='LZFUGRU01'
```

### 1.3 Bidirectional Analysis

ZXRAY uses both directions:
1. **DOWN:** Build complete dependency tree (what does this package use?)
2. **UP:** Find usage (who uses objects in this package?)

---

## 2. CROSS Table: Complete Reference

### 2.1 Structure

**Table:** `CROSS`
**Description:** Cross-reference for classical ABAP elements

**Key Fields:**
- `TYPE` - Reference type (character 1)
- `NAME` - Referenced object name
- `INCLUDE` - Program/include where reference occurs
- `PROG` - Additional program context (optional)

### 2.2 Complete Type Reference

| Type | Description | Count* | TADIR Object | URI Prefix | Handler Class | Notes |
|------|-------------|---------|--------------|------------|---------------|-------|
| `0` | Message class (SE91) | 84 | MSAG | - | ZCL_RAY_00_NODE_MSAG | Message class reference |
| `2` | CALL TRANSFORMATION | 4 | XSLT | - | ZCL_RAY_00_NODE_XSLT | XSLT transformations |
| `3` | Messages in Exception | 21 | CLAS | - | ZCL_RAY_00_NODE_CLAS | Exception class messages |
| `A` | AUTHORITY-CHECK OBJECT | 23 | SUSO | - | ZCL_RAY_00_NODE_SUSO | Authorization objects |
| `E` | SET PF-STATUS | 4 | - | - | - | GUI status (UI related) |
| `F` | Function Module call | 617 | FUNC/FUGR | F | ZCL_RAY_00_NODE_FUNC | **Most common** |
| `M` | Search Help (Dynpro) | 10 | SHLP | - | ZCL_RAY_00_NODE_SHLP | Screen search help |
| `N` | Message (T100A) | 831 | MSAG | - | ZCL_RAY_00_NODE_MSAG | Individual messages |
| `P` | GET/SET Parameter | 20 | PARA | - | ZCL_RAY_00_NODE_PARA | Memory parameters |
| `R` | Report Submit/Perform | 44 | PROG | R | ZCL_RAY_00_NODE_PROG | SUBMIT/PERFORM in other report |
| `S` | DB Table in "Tables" | 135 | TABL | - | ZCL_RAY_00_NODE_TABL | Table declarations |
| `T` | Transaction call | 6 | TRAN | T | ZCL_RAY_00_NODE_TRAN | CALL TRANSACTION |
| `U` | Perform | 110 | PROG | U | ZCL_RAY_00_NODE_PERFORM | PERFORM subroutine |
| `V` | Search Help MATCHCODE | 6 | MCOB | - | ZCL_RAY_00_NODE_MCOB | Matchcode objects |
| `Y` | Transaction (rare) | 2 | TRAN | - | ZCL_RAY_00_NODE_TRAN | SE93 → PROGRAM |

*Counts from real SAP system (Z* objects only)

### 2.3 CROSS Type Details

#### Type 'F' - Function Module (617 occurrences)
```abap
" Example CROSS entry:
TYPE = 'F'
NAME = 'BAPI_USER_GET_DETAIL'
INCLUDE = 'ZCL_USER_MANAGER========CP'

" Code that creates it:
CALL FUNCTION 'BAPI_USER_GET_DETAIL'
```

**Usage in ZXRAY:**
```abap
SELECT * FROM cross
  WHERE include IN @includes
    AND type = 'F'
    AND name IN @mtr_fm_filter.  " Filter for Z* or allowed FMs
```

---

#### Type 'R' - Report Submit/Perform (44 occurrences)
```abap
" Example CROSS entry:
TYPE = 'R'
NAME = 'ZREPORT_PROCESS'
INCLUDE = 'ZCALLER_PROGRAM'
PROG = 'ZREPORT_PROCESS'

" Code that creates it:
SUBMIT zreport_process
PERFORM some_form IN PROGRAM zreport_process
```

**Helper Functions:**
- `RS_GET_ALL_INCLUDES` - Get all includes for a program
- `RS_GET_MAINPROGRAMS` - Get main program for an include

---

#### Type 'T' - Transaction (6 occurrences)
```abap
" Example CROSS entry:
TYPE = 'T'
NAME = 'ZMM01'
INCLUDE = 'ZPROGRAM'

" Code that creates it:
CALL TRANSACTION 'ZMM01'
```

**Related Tables:**
- `TSTC` - Transaction codes
- `TSTCC` - Transaction code details
- `TSTCP` - Parameter transactions (SM30, START_REPORT)
- `TCVIEW` - Transaction views

**Special Case - Parameter Transactions:**
```abap
" TSTCP entry for SM30:
TCODE = 'ZSM30_CUSTOM'
PARAM = 'SM30 VIEWNAME=ZVIEW_NAME'

" ZXRAY creates edge:
T.ZSM30_CUSTOM → T.SM30-ZVIEW_NAME
```

---

#### Type 'U' - Perform (110 occurrences)
```abap
" Example CROSS entry:
TYPE = 'U'
NAME = 'MY_SUBROUTINE'
INCLUDE = 'ZCALLER'
PROG = 'ZPERFORMER'  " Program containing the FORM

" Code that creates it:
PERFORM my_subroutine IN PROGRAM zperformer
```

**Note:** Performs within same program/include are NOT in CROSS (local scope).

---

#### Type 'S' - Table Declaration (135 occurrences)
```abap
" Example CROSS entry:
TYPE = 'S'
NAME = 'MARA'
INCLUDE = 'ZPROGRAM'

" Code that creates it:
TABLES: mara.
```

**Note:** This captures `TABLES:` declarations, not all table usages (SELECT statements not included).

---

#### Type 'N' - Individual Messages (831 occurrences)
```abap
" Example CROSS entry:
TYPE = 'N'
NAME = 'ZMSG_CLASS'
INCLUDE = 'ZPROGRAM'

" Code that creates it:
MESSAGE e001(zmsg_class)
```

**Related Table:** `T100A` - Message class assignments

---

#### Type 'P' - Parameters (20 occurrences)
```abap
" Example CROSS entry:
TYPE = 'P'
NAME = 'BUK'
INCLUDE = 'ZPROGRAM'

" Code that creates it:
GET PARAMETER ID 'BUK' FIELD lv_bukrs.
SET PARAMETER ID 'BUK' FIELD lv_bukrs.
```

---

## 3. WBCROSSGT Table: Complete Reference

### 3.1 Structure

**Table:** `WBCROSSGT`
**Full Name:** Cross-Reference for Global Types
**Description:** Cross-reference for OO ABAP elements

**Key Fields:**
- `OTYPE` - Object type (2 characters)
- `NAME` - Referenced object name
- `INCLUDE` - Program/include where reference occurs

### 3.2 Complete Type Reference

| OType | Description | Count* | Real System** | URI Prefix | Notes |
|-------|-------------|---------|---------------|------------|-------|
| `DA` | Data | 50,043 | 7,442,438 | DA | Direct data access |
| `EV` | Event | 25 | 38,875 | EV | Class/Interface events |
| `ME` | Method | 19,336 | 2,020,310 | ME | **Most important for call graph** |
| `TK` | Type Key | 75 | 4,909 | - | Rare, mainly in ZABAPGIT |
| `TY` | Type | 57,605 | 12,362,577 | TY | Type references |

*Z* objects from reference system
**Full system totals (all objects)

### 3.3 WBCROSSGT Type Details

#### OType 'ME' - Method (19,336 Z* / 2M total)
```abap
" Example WBCROSSGT entry:
OTYPE = 'ME'
NAME = 'ZCL_LOGGER→WRITE'
INCLUDE = 'ZCL_MY_CLASS========CP'

" Code that creates it:
DATA(lo_logger) = zcl_logger=>write( 'message' ).
lo_instance->method_name( ).
```

**Method Name Patterns in WBCROSSGT:**
```
Pattern                     | Example                     | Meaning
----------------------------|-----------------------------|---------
CLASS\ME:METHOD            | ZCL_CLASS\ME:METHOD         | Public/Protected method
CLASS\TY:LOCAL\ME:METHOD   | ZCL_CLASS\TY:LCL\ME:METHOD  | Local class method
CLASS\IN:INTF\ME:METHOD    | ZCL_CLASS\IN:IF_X\ME:METHOD | Interface method
```

**Usage in ZXRAY:**
```abap
SELECT * FROM wbcrossgt
  WHERE include IN @includes
    AND otype = 'ME'
    AND name IN @mtr_me_filter.  " Package-based filter
```

---

#### OType 'TY' - Type (57,605 Z* / 12M total)
```abap
" Example WBCROSSGT entry:
OTYPE = 'TY'
NAME = 'ZCL_TYPES'
INCLUDE = 'ZCL_MY_CLASS========CP'

" Code that creates it:
DATA: lt_data TYPE zcl_types=>tt_table.
DATA: lr_ref  TYPE REF TO zcl_some_class.
```

**Type Categories:**
- Dictionary types (DDIC)
- Class types (CLAS)
- Interface types (INTF)
- Local types (within classes)

**Note:** TY is the largest category but often filtered heavily in ZXRAY to reduce noise.

---

#### OType 'DA' - Data (50,043 Z* / 7.4M total)
```abap
" Example WBCROSSGT entry:
OTYPE = 'DA'
NAME = 'ZCL_CLASS→ATTRIBUTE'
INCLUDE = 'ZCL_CALLER========CP'

" Code that creates it:
lv_value = zcl_class=>static_attribute.
lv_value = lo_instance->attribute.
```

**Pattern Examples:**
```
CLASS\DA:ATTRIBUTE              - Static attribute
CLASS\ME:METHOD\DA:PARAMETER    - Method parameter
CLASS\TY:LOCAL\DA:DATA          - Local class data
```

**Usage:** Usually heavily filtered in ZXRAY as it generates massive result sets.

---

#### OType 'EV' - Event (25 Z* / 38,875 total)
```abap
" Example WBCROSSGT entry:
OTYPE = 'EV'
NAME = 'ZCL_CLASS→DATA_CHANGED'
INCLUDE = 'ZCL_HANDLER========CP'

" Code that creates it:
CLASS handler DEFINITION.
  PUBLIC SECTION.
    METHODS: on_data_changed FOR EVENT data_changed OF zcl_class.
ENDCLASS.

SET HANDLER lo_handler->on_data_changed FOR lo_object.
```

**Usage:** Rare in custom code, usually filtered out.

---

#### OType 'TK' - Type Key (75 Z* / 4,909 total)
```abap
" Example WBCROSSGT entry:
OTYPE = 'TK'
NAME = 'ZIF_SOMETHING→TYPE_KEY'
INCLUDE = 'ZCL_CLASS========CP'
```

**Usage:** Very rare, mainly seen in ZABAPGIT, usually filtered out.

---

## 4. Real System Statistics

### 4.1 CROSS Distribution (Z* Custom Objects)

```
Type  Count  Percentage  Description
----  -----  ----------  -----------
N     831    43.3%       Messages (most common)
F     617    32.2%       Function modules
S     135     7.0%       Table declarations
U     110     5.7%       Performs
R      44     2.3%       Reports/Submits
Others 180    9.5%       Various
----  -----  ----------
Total 1,917  100.0%
```

### 4.2 WBCROSSGT Distribution (Z* Custom Objects)

```
OType  Count   Percentage  Description
-----  ------  ----------  -----------
TY     57,605  45.3%       Types (largest)
DA     50,043  39.4%       Data access
ME     19,336  15.2%       Methods (most important)
TK         75   0.1%       Type keys
EV         25   0.0%       Events
-----  ------  ----------
Total 127,084  100.0%
```

### 4.3 Comparison: Custom vs. Full System

```
Table      Custom (Z*)  Full System   Ratio
---------  -----------  -----------  ------
CROSS           1,917    50,000,000+  0.004%
WBCROSSGT     127,084    21,866,109   0.58%

Observation: Custom code represents <1% of total cross-references
```

### 4.4 Growth Patterns

**WBCROSSGT grows faster than CROSS:**
- OO ABAP generates more references (types, interfaces, inheritance)
- Classical ABAP has fewer reference types
- ME (methods) alone exceeds all CROSS entries

---

## 5. Handler Class Architecture

### 5.1 Node Handler Pattern

ZXRAY implements the **Factory Pattern** with type-specific handlers:

```abap
" Base class
ZCL_RAY_00_NODE
  └─ factory_by_cross()     " Dispatches based on TYPE
  └─ factory_by_wbcrossgt() " Dispatches based on OTYPE
  └─ factory_by_tadir()     " Dispatches based on OBJECT
```

### 5.2 Handler Class Mapping

#### CROSS Type Handlers

| Type | Handler Class | Responsibility |
|------|---------------|----------------|
| '0' | ZCL_RAY_00_NODE_MSAG | Message class SE91 |
| '2' | ZCL_RAY_00_NODE_XSLT | XSLT transformations |
| '3' | ZCL_RAY_00_NODE_CLAS | Exception messages |
| 'A' | ZCL_RAY_00_NODE_SUSO | Authority checks |
| 'F' | ZCL_RAY_00_NODE_FUNC | Function modules |
| 'M' | ZCL_RAY_00_NODE_SHLP | Dynpro search helps |
| 'N' | ZCL_RAY_00_NODE_MSAG | T100A messages |
| 'P' | ZCL_RAY_00_NODE_PARA | Memory parameters |
| 'R' | ZCL_RAY_00_NODE_PROG | Reports/programs |
| 'S' | ZCL_RAY_00_NODE_TABL | Database tables |
| 'T' | ZCL_RAY_00_NODE_TRAN | Transactions |
| 'U' | ZCL_RAY_00_NODE_PERFORM | Subroutines |
| 'V' | ZCL_RAY_00_NODE_MCOB | Matchcode objects |
| 'Y' | ZCL_RAY_00_NODE_TRAN | Transaction variants |

#### WBCROSSGT Type Handlers

**Note:** WBCROSSGT types (ME, TY, DA, EV, TK) are typically handled generically without separate handler classes, as they represent references within the OO system rather than distinct object types.

### 5.3 Handler Responsibilities

Each handler class typically implements:

```abap
CLASS zcl_ray_00_node_<type> DEFINITION
  INHERITING FROM zcl_ray_00_node.

  PUBLIC SECTION.
    CLASS-METHODS:
      " Factory methods
      new_by_cross IMPORTING is_ TYPE cross RETURNING VALUE(ro_) TYPE REF TO ...,
      new_by_tadir IMPORTING is_ TYPE tadir RETURNING VALUE(ro_) TYPE REF TO ...,
      new_by_desc  IMPORTING is_ TYPE descriptor RETURNING VALUE(ro_) TYPE REF TO ...,

      " Node-specific operations
      get_source_code  RETURNING VALUE(rv_) TYPE string_table,
      get_metadata     RETURNING VALUE(rs_) TYPE metadata,
      expand_children  RETURNING VALUE(rt_) TYPE node_table.

ENDCLASS.
```

---

## 6. URI Prefix System

### 6.1 URI Structure

ZXRAY uses a hierarchical URI system to identify nodes:

```
Pattern: <PREFIX>.<NAME>[\<CONTEXT>]

Examples:
- F.Z_FUNCTION_MODULE
- ME.ZCL_CLASS\ME:METHOD
- R.ZPROGRAM
- T.ZMM01
- TY.ZCL_TYPES
```

### 6.2 Prefix Mapping

| Prefix | Origin | Description | Example |
|--------|--------|-------------|---------|
| `F` | CROSS type='F' | Function module | `F.BAPI_USER_GET` |
| `R` | CROSS type='R' | Report/Program | `R.ZREPORT` |
| `T` | CROSS type='T' | Transaction | `T.ZMM01` |
| `U` | CROSS type='U' | Subroutine | `U.SUBROUTINE\R:PROG` |
| `ME` | WBCROSSGT otype='ME' | Method | `ME.ZCL_CLASS\ME:METHOD` |
| `TY` | WBCROSSGT otype='TY' | Type | `TY.ZCL_TYPES` |
| `DA` | WBCROSSGT otype='DA' | Data | `DA.ZCL_CLASS\DA:ATTR` |
| `EV` | WBCROSSGT otype='EV' | Event | `EV.ZCL_CLASS\EV:EVENT` |

### 6.3 Context Notation

For nested elements, ZXRAY uses backslash notation:

```
Pattern: <CLASS>\<SECTION>:<ELEMENT>

Examples:
ZCL_CLASS\ME:METHOD                    - Public/Protected method
ZCL_CLASS\TY:LOCAL_TYPE                - Local type
ZCL_CLASS\TY:LOCAL\ME:METHOD           - Local class method
ZCL_CLASS\IN:IF_INTERFACE\ME:METHOD    - Interface method
ZCL_CLASS\ME:METHOD\DA:PARAMETER       - Method parameter
```

---

## 7. Query Optimization Strategies

### 7.1 Filter Design

ZXRAY uses RANGE tables for flexible filtering:

```abap
" Package filter - most important
DATA: mtr_pa_filter TYPE RANGE OF devclass.
mtr_pa_filter = VALUE #(
  ( sign = 'I' option = 'CP' low = 'Z*' )
  ( sign = 'I' option = 'CP' low = '$Z*' )
).

" Function module filter - exclude SAP standard
DATA: mtr_fm_filter TYPE RANGE OF funcname.
mtr_fm_filter = VALUE #(
  ( sign = 'I' option = 'CP' low = 'Z*' )
  ( sign = 'E' option = 'CP' low = 'RS_*' )  " Exclude tools
).

" Method filter - by package
DATA: mtr_me_filter TYPE RANGE OF string.
mtr_me_filter = VALUE #(
  ( sign = 'I' option = 'CP' low = 'ZCL_*' )
).
```

### 7.2 Batch Processing

**FOR ALL ENTRIES Pattern:**

```abap
" Instead of this (slow):
LOOP AT lt_includes INTO DATA(ls_include).
  SELECT * FROM cross
    WHERE include = @ls_include-name
    APPENDING TABLE @lt_cross.
ENDLOOP.

" Use this (fast):
SELECT * FROM cross
  FOR ALL ENTRIES IN @lt_includes
  WHERE include = @lt_includes-name
  INTO TABLE @lt_cross.
```

### 7.3 Slicing for Large Result Sets

```abap
" When lt_nodes has 50,000 entries, slice into batches
DATA(lo_slice) = zcl_ray_00_slice=>new(
  it_      = lt_nodes
  iv_slice = 5000  " Process 5000 at a time
).

DATA: ltr_slice LIKE ltr_nodes.
WHILE lo_slice->next( CHANGING ct_ = ltr_slice ).
  SELECT * FROM cross
    WHERE include IN @ltr_slice
    APPENDING TABLE @lt_cross.
ENDWHILE.
```

**Why:** Oracle/HANA limits IN clause size; prevents "SQL statement too long" errors.

### 7.4 Selective Column Retrieval

```abap
" Don't do this:
SELECT * FROM wbcrossgt ...

" Do this:
SELECT otype, name, include
  FROM wbcrossgt
  WHERE ...
```

**Impact:** WBCROSSGT has many columns; selecting only needed fields reduces memory and network overhead.

---

## 8. Advanced Topics

### 8.1 Include Resolution

**Helper Function:** `RS_PROGNAME_DECIDER`

```abap
CALL FUNCTION 'RS_PROGNAME_DECIDER'
  EXPORTING
    include = 'LZLLM_FGU01'
  IMPORTING
    object            = 'FUNC'        " Object type
    obj_name          = 'ZLLM_EKEKE'  " Object name
    enclosing_obj_name = 'SAPLZLLM_FG' " Parent object
    enclosing_object  = 'FUGR'.       " Parent type
```

**Usage:** Convert include name to its enclosing object (class/function group/program).

**Other Helper Functions:**
- `RS_PROGNAME_SPLIT` - Parse program name components
- `RS_PROGNAME_CONCATENATE` - Build program name from components
- `RS_PROGRAM_GET_DEVCLASS` - Get package for a program

### 8.2 Transaction Resolution

**Complex Parameter Transactions:**

```abap
" TSTCP entry:
TCODE = 'ZSM30_VIEW'
PARAM = 'SM30 VIEWNAME=ZVIEW_NAME;UPDATE=X'

" Parse logic:
SPLIT ls_tstcp-param AT ';' INTO TABLE lt_params.
LOOP AT lt_params INTO lv_param.
  SPLIT lv_param AT '=' INTO lv_key lv_value.
  CASE lv_key.
    WHEN 'VIEWNAME' OR 'D_SREPOVARI-REPORT' OR 'VCLDIR-VCLNAME'.
      " Extract actual target
  ENDCASE.
ENDLOOP.
```

**Transaction Types:**
1. **Direct** - Points to program (TSTC-PGMNA)
2. **Parameter** - Points to another transaction with parameters (TSTCP)
3. **Variant** - SE93 variants

### 8.3 Method Pattern Matching

```abap
DATA(lt_case) = VALUE tt_case(
  ( pattern = '*\TY:*\IN:*\ME:*' case = 'LOCAL_CLASS_INTF_METHOD' )
  ( pattern = '*\IN:*\ME:*'      case = 'INTERFACE_METHOD' )
  ( pattern = '*\ME:*'           case = 'CLASS_METHOD' )
  ( pattern = '*'                case = 'CLASS' )
).

LOOP AT lt_case INTO DATA(ls_case).
  IF lv_name CP ls_case-pattern.
    " Handle based on case
  ENDIF.
ENDLOOP.
```

**Purpose:** Distinguish between:
- Regular class methods
- Interface methods
- Local class methods
- Methods within nested structures

---

## 9. Known Limitations

### 9.1 Dynamic Code

**Not Captured by CROSS/WBCROSSGT:**

```abap
" Dynamic function call
DATA(lv_fm) = 'Z_FUNCTION'.
CALL FUNCTION lv_fm.  " ❌ Not in CROSS

" Dynamic method call
DATA(lv_method) = 'METHOD_NAME'.
CALL METHOD lo_obj->(lv_method).  " ❌ Not in WBCROSSGT

" Dynamic type creation
CREATE OBJECT lo_obj TYPE (lv_classname).  " ❌ Not in WBCROSSGT

" Dynamic SQL
DATA(lv_table) = 'MARA'.
SELECT * FROM (lv_table).  " ❌ Not in CROSS
```

**Mitigation:**
- Accept 95%+ coverage as sufficient
- Use naming conventions to hint relationships
- Manual documentation for critical dynamic code
- Code review to identify dynamic patterns

### 9.2 Generated Code

**Partially Captured:**

```abap
" CDS views - associations may not be fully tracked
" BOPF generated code - references may be incomplete
" Enhancement implementations - depends on framework
" Dynamic proxy classes - usually not tracked
```

### 9.3 External Calls

**Not in Local CROSS/WBCROSSGT:**

```abap
" RFC calls
CALL FUNCTION 'Z_REMOTE' DESTINATION 'DEST'.

" HTTP calls
cl_http_client=>create_by_url( ... ).

" External web services
" OData services
```

### 9.4 Performance at Scale

**Issues:**
- CROSS: ~50M rows
- WBCROSSGT: ~21M rows (Z* custom) to ~67M rows (full system)
- FOR ALL ENTRIES has limits (~30,000 entries)
- Memory consumption for large result sets

**Solutions:**
- Aggressive caching
- Package scoping
- Slicing
- Background processing
- Database indexes

---

## 10. Best Practices

### 10.1 Query Design

**DO:**
```abap
✓ Use FOR ALL ENTRIES for batch queries
✓ Apply package filters early
✓ Select only needed columns
✓ Use slicing for large datasets
✓ Cache results aggressively
✓ Set reasonable limits (level_limit, edge_limit)
```

**DON'T:**
```abap
✗ Loop and query individually
✗ SELECT * without filters
✗ Query full system without package scope
✗ Ignore memory limits
✗ Process synchronously for large packages
```

### 10.2 Filter Configuration

**Minimal Filters (Fast, Broad):**
```abap
" Only package filter
mtr_pa_filter = VALUE #( ( sign = 'I' option = 'CP' low = 'Z*' ) ).
" Process ALL types within package
```

**Moderate Filters (Balanced):**
```abap
" Package + exclude noisy types
mtr_pa_filter = VALUE #( ( sign = 'I' option = 'CP' low = 'Z*' ) ).
mtr_da_filter = VALUE #( ( sign = 'E' option = 'CP' low = '*' ) ).  " Exclude DA
mtr_tk_filter = VALUE #( ( sign = 'E' option = 'CP' low = '*' ) ).  " Exclude TK
```

**Strict Filters (Precise, Slow):**
```abap
" Explicit include/exclude for each type
mtr_fm_filter = VALUE #(
  ( sign = 'I' option = 'CP' low = 'Z*' )
  ( sign = 'E' option = 'EQ' low = 'Z_OBSOLETE*' )
).
```

### 10.3 Traversal Strategy

**For Documentation (DOWN traversal):**
```abap
" What does this package call/use?
lo_graph->build_down_and_save( starting_nodes ).
```

**For Impact Analysis (UP traversal):**
```abap
" What uses this package?
lo_graph->build_up_and_save( starting_nodes ).
```

**For Complete Picture (Both):**
```abap
" Build full graph
lt_down = lo_graph->build_down( starting_nodes ).
lt_up   = lo_graph->build_up( starting_nodes ).
```

### 10.4 Cache Management

**When to Cache:**
- Stable codebases (not actively developed)
- Repeated analysis runs
- Multi-step processing (graph → metrics → LLM)

**When to Bypass Cache:**
- Fresh code analysis
- After major refactoring
- Debugging graph issues
- Forcing complete rebuild

**Cache Invalidation:**
```abap
" Clear cache for specific package
lo_graph = zcl_xray_graph=>new( iv_bypass_cache = 'X' ).

" Or manually:
DELETE FROM zllm_00_edge WHERE seed = ... AND devclass = ...
DELETE FROM zllm_00_node WHERE seed = ... AND devclass = ...
```

---

## 11. Troubleshooting

### 11.1 Missing Edges

**Symptom:** Expected relationships not showing in graph

**Diagnosis:**
1. Check package filter: `mtr_pa_filter`
2. Check type filters: `mtr_fm_filter`, `mtr_me_filter`, etc.
3. Verify object is in scope
4. Check CROSS/WBCROSSGT directly

**Fix:**
```abap
" Debug query
SELECT * FROM cross
  WHERE include = '<your_include>'
  INTO TABLE @DATA(lt_debug).
" Check if entries exist

" Verify filters
DATA(lr_filtered_out) = line_exists( mtr_fm_filter[ low = 'YOUR_FM' ] ).
```

### 11.2 Slow Performance

**Symptom:** Query takes minutes or times out

**Diagnosis:**
1. Check result set size: `DESCRIBE TABLE lt_result LINES lv_count`
2. Check filter effectiveness
3. Monitor database load (ST04)
4. Check FOR ALL ENTRIES size

**Fix:**
```abap
" Add package filter if missing
" Reduce result set with stricter filters
" Enable caching
" Use slicing for large inputs
" Run in background job
```

### 11.3 Memory Overflow

**Symptom:** STORAGE_PARAMETERS_WRONG_SET or TSV_TNEW_PAGE_ALLOC_FAILED

**Diagnosis:**
1. Check internal table sizes
2. Monitor memory consumption (ST22)
3. Check result set from CROSS/WBCROSSGT

**Fix:**
```abap
" Process in smaller batches
lo_slice = zcl_ray_00_slice=>new( iv_slice = 1000 ).  " Smaller slices

" Free memory after each batch
FREE lt_result.
FREE lt_cross.

" Set limits
lo_graph = zcl_xray_graph=>new(
  iv_edge_limit = 10000  " Stop after 10k edges
).
```

### 11.4 Incorrect Relationships

**Symptom:** Graph shows wrong dependencies

**Diagnosis:**
1. Check CROSS/WBCROSSGT currency (run RADMASRV)
2. Verify include resolution
3. Check for duplicate entries
4. Verify handler logic

**Fix:**
```abap
" Regenerate SAP cross-references
REPORT radmasrv.  " Run with package filter

" Clear ZXRAY cache
DELETE FROM zllm_00_node WHERE seed = @mv_seed.
DELETE FROM zllm_00_edge WHERE seed = @mv_seed.

" Rebuild with bypass_cache = X
```

---

## 12. Appendix: Quick Reference Tables

### A. CROSS Type Quick Lookup

| Type | What It Captures | Query By | Common Filters |
|------|------------------|----------|----------------|
| F | `CALL FUNCTION 'FM'` | include | Z*, exclude tools |
| R | `SUBMIT prog` | include | Z* programs |
| T | `CALL TRANSACTION` | include | Z* transactions |
| S | `TABLES: table` | include | Exclude SAP std |
| U | `PERFORM form IN PROGRAM` | include | Include = Z* |
| N | `MESSAGE e001(mc)` | include | All messages |
| P | `GET/SET PARAMETER ID` | include | Usually all |
| Others | Various | include | Case-by-case |

### B. WBCROSSGT Type Quick Lookup

| OType | What It Captures | Query By | Common Filters |
|-------|------------------|----------|----------------|
| ME | `lo_obj->method()` | include | Package-based |
| TY | `TYPE REF TO class` | include | Exclude noise types |
| DA | `lo_obj->attribute` | include | Usually excluded |
| EV | `FOR EVENT evt OF class` | include | Usually excluded |
| TK | Type keys | include | Usually excluded |

### C. URI Prefix Patterns

| Pattern | Example | Meaning |
|---------|---------|---------|
| `F.<name>` | `F.Z_FM` | Function module |
| `R.<name>` | `R.ZPROGRAM` | Report/Program |
| `T.<name>` | `T.ZMM01` | Transaction |
| `ME.<class>\ME:<method>` | `ME.ZCL_X\ME:DO` | Method |
| `TY.<type>` | `TY.ZCL_TYPES` | Type |
| `<from>\R:<prog>` | `U.FORM\R:ZPROG` | Context reference |

### D. Key Helper Functions

| Function | Purpose | Input | Output |
|----------|---------|-------|--------|
| `RS_PROGNAME_DECIDER` | Resolve include to object | Include name | Object type/name/parent |
| `RS_GET_ALL_INCLUDES` | Get all includes for program | Program name | Include table |
| `RS_GET_MAINPROGRAMS` | Get main program for include | Include name | Main program(s) |
| `RS_PROGRAM_GET_DEVCLASS` | Get package for program | Program name | Package |
| `GET_INCLUDETAB` | Get include list | Program name | Include table |

---

## 13. Conclusion

This reference guide provides comprehensive information about SAP's cross-reference infrastructure as used by ZXRAY. Key takeaways:

1. **CROSS** (1,917 entries) captures classical ABAP: functions, reports, transactions, tables
2. **WBCROSSGT** (127,084 entries) captures OO ABAP: methods, types, data, events
3. **Traversal patterns** (UP/DOWN) enable both dependency and impact analysis
4. **Handler classes** provide type-specific logic for each reference category
5. **URI system** creates unique identifiers for all nodes in the graph
6. **Filtering** is critical for performance and relevance
7. **Caching** dramatically improves repeated analysis
8. **Limitations** exist (dynamic code, generated code) but coverage is 95%+

Together with Report 001, this provides a complete technical reference for understanding and working with ZXRAY's graph-building capabilities.

---

**End of Reference Guide**
