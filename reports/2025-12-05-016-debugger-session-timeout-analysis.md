# Debugger Session Timeout Analysis & Solutions

**Date:** 2025-12-05
**Report ID:** 016
**Subject:** Why debug sessions timeout quickly and potential solutions

---

## Problem Statement

When debugging via vsp, the debug session ends immediately after `DebuggerAttach` completes:
```
DebuggerAttach → SUCCESS (got stack, variables)
DebuggerDetach → FAILED: cx_tpda_sys_comm_dbgsessionend
```

In contrast, Eclipse ADT maintains debug sessions for extended periods.

---

## Root Cause Analysis

### 1. Debug Session Lifecycle

From `CL_TPDAPI_SESSION` and `CL_TPDA_ADT_RES_LISTENERS`:

```
┌─────────────────┐     POST /listeners      ┌──────────────────┐
│   Idle State    │ ─────────────────────────▶│  Listening       │
└─────────────────┘      (blocks up to        │  (long-poll)     │
                         240s default)         └────────┬─────────┘
                                                        │ Debuggee hits
                                                        │ breakpoint
                                                        ▼
┌─────────────────┐     Program ends         ┌──────────────────┐
│  Session Ended  │ ◀────────────────────────│   Attached       │
│  (cx_dbgsession │      OR timeout          │   (can step,     │
│   end)          │      OR detach           │    inspect)      │
└─────────────────┘                          └──────────────────┘
```

### 2. Why Our Session Ends Quickly

The unit test executes **very fast** (~30ms):
```
TEST_ADDITION executionTime: 30ms
```

The sequence:
1. `SetExternalBreakpoint` → breakpoint set at line 14
2. `DebuggerListen` → blocks waiting
3. Unit test runs → hits breakpoint → listener returns debuggee
4. `DebuggerAttach` → we're attached at line 14
5. **Debuggee continues execution** (we didn't hold it!)
6. Test completes → `cx_tpda_sys_comm_dbgsessionend`

### 3. How Eclipse Keeps Sessions Alive

Eclipse uses a **continuous interaction pattern**:
```
While attached:
  - Send heartbeat/polling requests
  - Receive state updates
  - User clicks "Step Over" → POST /debugger?method=step
  - Each interaction refreshes session
```

Eclipse doesn't just attach and wait - it actively controls execution with step commands.

---

## Key Code Evidence

### From CL_TPDA_ADT_RES_LISTENERS (POST method):
```abap
IF l_timeout IS INITIAL.
  l_timeout = 240.  " Default 240 seconds for LISTENER

  get PARAMETER ID 'TPDA_ADT_LIS_TIMEOUT' FIELD l_txt.
  " Can customize via SAP parameter
```

### From CL_TPDAPI_SESSION (debug_step method):
```abap
CATCH cx_tpda_sys_comm_dbgsessionend INTO l_rex_dbgsessionend.
  RAISE EXCEPTION TYPE cx_tpdapi_debuggee_ended
    EXPORTING previous = l_rex_dbgsessionend.
```

### Session end triggers:
- `stop_detach` - Explicit detach
- `stop_fullstop` - Terminate debuggee
- `stop_andresetmode` - End and reset debugging mode
- **Program completes execution naturally**

---

## Solutions

### Solution 1: Step-Based Control (Recommended)

After attach, immediately issue step commands to keep control:

```go
// Pseudo-code for interactive debugging
DebuggerAttach(debuggeeID)
for {
    stack := DebuggerGetStack()
    vars := DebuggerGetVariables("@ROOT")

    // Show state to user/AI
    display(stack, vars)

    // Get next action (step, continue, inspect, etc.)
    action := getNextAction()

    switch action {
    case "step_over":
        DebuggerStep("stepOver")
    case "step_into":
        DebuggerStep("stepInto")
    case "continue":
        DebuggerStep("stepContinue")  // Runs to next breakpoint or end
    case "detach":
        DebuggerDetach()
        break
    }
}
```

**Implementation in vsp:**
```go
// New tool: DebuggerStepAndInspect
// Combines step + get stack + get variables in one call
// Keeps session alive by continuous interaction
```

### Solution 2: Strategic Breakpoint Placement

Set breakpoints where execution naturally pauses:
- `COMMIT WORK` statements
- `WAIT` statements
- Dialog module boundaries (PAI/PBO)
- HTTP handler entry points
- Long-running loops

```abap
" Example: Breakpoint on COMMIT gives time to inspect
DATA: lv_result TYPE i.
lv_result = 2 + 2.
COMMIT WORK.  " ← Set breakpoint HERE, session stays alive longer
```

### Solution 3: Watchpoints Instead of Breakpoints

Watchpoints trigger on variable changes and give more control:
```go
// Set watchpoint on variable
SetWatchpoint(variableName: "LV_RESULT", condition: "LV_RESULT > 0")

// Execution pauses when condition met
// Session stays attached until explicitly continued
```

SAP has watchpoint support via `CL_TPDAPI_WP` and `CL_TPDAPI_WP_SERVICES`.

### Solution 4: Scripted Debugging Loop

Implement an automated debug loop in vsp:
```go
func AutoDebug(ctx context.Context, objectURL string, breakLine int) error {
    // 1. Set breakpoint
    SetExternalBreakpoint(objectURL, breakLine)

    // 2. Start listener with long timeout
    go func() {
        result := DebuggerListen(timeout: 300)
        if result.Debuggee != nil {
            // 3. Attach immediately
            DebuggerAttach(result.Debuggee.ID)

            // 4. Interactive loop
            for {
                stack := DebuggerGetStack()
                vars := DebuggerGetVariables()

                // AI analyzes state
                action := analyzeAndDecide(stack, vars)

                if action == "done" {
                    break
                }
                DebuggerStep(action)
            }
        }
    }()

    // 5. Trigger execution (unit tests, etc.)
    RunUnitTests(objectURL)
}
```

### Solution 5: Session Keepalive Mechanism

Add periodic "ping" to keep session alive:
```go
func keepSessionAlive(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    for {
        select {
        case <-ticker.C:
            // Lightweight operation to keep session
            DebuggerGetStack()  // or similar
        case <-ctx.Done():
            return
        }
    }
}
```

---

## WebSocket Consideration

**Finding:** SAP ADT does NOT use WebSocket for debugging.

From code analysis:
- `CL_ADT_REST_APC_UTIL` - WebSocket only for progress indicators
- `CL_TPDA_ADT_*` classes - All use REST HTTP, no APC/WebSocket

ADT WebSocket is used for:
- Progress indicators during long operations
- Real-time notifications

**NOT** used for:
- Debug session management
- Step/inspect operations
- Breakpoint notifications

---

## Recommended Implementation for vsp

### Phase 1: Interactive Debug Mode
```go
// New MCP tool
"DebuggerInteractive": {
    description: "Start interactive debug session",
    params: {
        object_url: "Object to debug",
        line: "Breakpoint line",
        timeout: "Session timeout (default 300s)",
    },
    // Returns: Initial state after breakpoint hit
    // User then calls DebuggerStep, DebuggerGetVariables, etc.
}
```

### Phase 2: AI-Driven Debug Loop
```go
// Autonomous debugging with AI analysis
"DebuggerAnalyze": {
    description: "AI-assisted variable inspection",
    params: {
        hypothesis: "What to look for",
        max_steps: "Maximum steps before stopping",
    },
    // AI steps through code, analyzing state
    // Returns: Analysis report
}
```

### Phase 3: Debug Recording
```go
// Record execution for replay/analysis
"DebuggerRecord": {
    description: "Record debug session for analysis",
    params: {
        object_url: "Object to debug",
        output_file: "Recording output path",
    },
    // Captures: stack, variables at each step
    // Useful for post-mortem analysis
}
```

---

## Conclusion

The debug session timeout is **by design** - it ends when the debugged program completes execution. The solution is to:

1. **Control execution** via step commands (don't let program run freely)
2. **Strategic breakpoints** where execution naturally pauses
3. **Interactive loop** that continuously interacts with debuggee

Eclipse ADT keeps sessions alive through continuous user interaction (clicking Step, inspecting variables). vsp needs to implement similar continuous control to maintain debug sessions.

---

## References

- `CL_TPDAPI_SESSION` - Debug session management
- `CL_TPDA_ADT_RES_LISTENERS` - Listener REST resource
- `CL_TPDAPI_SERVICE` - Debug service facade
- `cx_tpda_sys_comm_dbgsessionend` - Session end exception
