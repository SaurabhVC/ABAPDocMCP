# Debugger Experiment Findings

**Date:** 2025-12-05
**Report ID:** 015
**Subject:** External Debugger API Testing and terminalId Issue

---

## Experiment Summary

Tested the ADT Debugger REST API through MCP tools to enable AI-driven debugging without SAP GUI interaction.

## What Worked

1. **SetExternalBreakpoint** - Successfully creates breakpoints:
   - Line breakpoints (with URI + line number)
   - Statement breakpoints (e.g., "CREATE OBJECT", "WRITE")
   - Returns valid breakpoint IDs

2. **DebuggerListen** - Successfully listens for debuggees:
   - Timeout mechanism works (tested 15-20 second timeouts)
   - Returns proper timeout message when no debuggee caught

3. **Test Class Creation** - Successfully created `ZCL_ADT_DEBUG_TEST`:
   - Class with `run_test` method
   - Test class `LCL_TEST` with `test_debug` method
   - Activation successful, tests pass

## Critical Issue: terminalId Mismatch

### Problem
Breakpoints set via `SetExternalBreakpoint` are not visible when querying with `GetExternalBreakpoints`.

### Root Cause
The ADT Debugger API uses `terminalId` to identify the IDE/tool session. Each breakpoint is scoped to a specific terminalId. The TypeScript MCP server (mcp-abap-adt) appears to generate a **new terminalId for each API call**, causing:

1. Breakpoint set with terminalId `A`
2. Query/Listen uses terminalId `B`
3. Breakpoints from `A` are invisible to `B`

### Evidence
```
SetExternalBreakpoint → Returns success with ID
GetExternalBreakpoints → Returns "No external breakpoints found"
```

This happens immediately after setting, with no intervening operations.

## Additional Issues

### RunUnitTests Parsing Error
```
parsing unit test results: strconv.ParseInt: parsing "0.1": invalid syntax
```
The execution time "0.1" (seconds) is being parsed as integer instead of float.

### HTTP Client Timeout
Long-polling requests (DebuggerListen with 30+ second timeout) fail with:
```
context deadline exceeded (Client.Timeout exceeded while awaiting headers)
```
The HTTP client has a default 30-second timeout that's shorter than the debugger's long-poll timeout.

## Required Fixes

### 1. Consistent terminalId (Critical)
Generate terminalId once per MCP server session, reuse for all debugger API calls.

**Go vsp pattern (correct):**
```go
var (
    terminalID     string
    terminalIDOnce sync.Once
)

func getTerminalID() string {
    terminalIDOnce.Do(func() {
        terminalID = "vsp-" + randomHex()
    })
    return terminalID
}
```

### 2. HTTP Timeout for Long-Polling
For DebuggerListen, use an HTTP client with extended timeout:
```go
// At least timeout + 30 seconds buffer
httpTimeout := time.Duration(opts.TimeoutSeconds+30) * time.Second
```

### 3. Float Parsing for Execution Time
Parse execution time as float64, not int:
```go
executionTime, _ := strconv.ParseFloat(timeStr, 64)
```

## Workaround (Manual Testing)

Until fixed, debugging can be tested manually:

1. Set breakpoint in SAP GUI (SE80/SE24)
2. Start DebuggerListen via MCP
3. Run unit tests in SAP GUI (SE24 → ZCL_ADT_DEBUG_TEST → Test → Unit Test)
4. DebuggerListen catches the debuggee
5. Use DebuggerAttach, DebuggerGetStack, DebuggerGetVariables

## Test Artifacts Created

| Object | Type | Package | Description |
|--------|------|---------|-------------|
| ZCL_ADT_DEBUG_TEST | CLAS | $TMP | Debug test class with test method |

## Next Steps

1. Fix terminalId persistence in TypeScript mcp-abap-adt
2. Fix HTTP timeout for long-polling requests
3. Fix execution time parsing (float vs int)
4. Re-test the full debug workflow
5. Document working debug workflow in CLAUDE.md

## Conclusion

The ADT Debugger REST API is functional and capable of supporting AI-driven debugging. The main blocker is the terminalId session management in the MCP server implementation. Once fixed, this will enable:

- AI setting breakpoints on suspicious code
- AI triggering execution via unit tests
- AI attaching and inspecting variables
- AI-powered root cause analysis
