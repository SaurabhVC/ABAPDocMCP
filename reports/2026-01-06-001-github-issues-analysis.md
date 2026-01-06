# GitHub Issues Analysis & Reflection

**Date:** 2026-01-06
**Report ID:** 001
**Subject:** Analysis of open GitHub issues from external users
**Status:** Active investigation

---

## Summary

Two open issues from **@marcellourbani** (Marcello Urbani), the first external user engagement on the project!

| Issue | Title | Created | Priority |
|-------|-------|---------|----------|
| #1 | Self-signed certificate | 2026-01-05 16:08 | High |
| #2 | GUI debugger | 2026-01-05 16:18 | Medium |

---

## Issue #1: Self-signed Certificate

### User Report
> "Is there any way to make it work, short of setting up a proxy? I tried GOINSECURE but no luck"

### Root Cause Analysis

**BUG CONFIRMED**: The `--insecure` flag is broken for WebSocket connections.

The HTTP client correctly implements TLS skip verification:
```go
// pkg/adt/config.go:199
TLSClientConfig: &tls.Config{
    InsecureSkipVerify: c.InsecureSkipVerify,  // ✅ Works for HTTP
},
```

But BOTH WebSocket clients **ignore** the `insecure` parameter:

```go
// pkg/adt/debug_websocket.go:120-122
dialer := websocket.Dialer{
    HandshakeTimeout: 30 * time.Second,
    // ❌ MISSING: TLSClientConfig!
}

// pkg/adt/amdp_websocket.go:115-117
dialer := websocket.Dialer{
    HandshakeTimeout: 30 * time.Second,
    // ❌ MISSING: TLSClientConfig!
}
```

### Fix Required

Both WebSocket clients need to add TLS configuration:

```go
dialer := websocket.Dialer{
    HandshakeTimeout: 30 * time.Second,
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: c.insecure,  // ADD THIS
    },
}
```

### Files to Modify
- `pkg/adt/debug_websocket.go:120-122`
- `pkg/adt/amdp_websocket.go:115-117`

### Impact
- **Severity**: High - Blocks all HTTPS SAP systems with self-signed certs
- **Effort**: 15 minutes
- **Risk**: None - simple fix

---

## Issue #2: GUI Debugger

### User Report
> "I tried to debug an ALV report but didn't work. Claude does set a breakpoint but doesn't hit it when I run it in sapgui or I ask it to run it. The websocket endpoint works fine."

### Root Cause Analysis

This is a **known limitation** documented in our reports. The issue relates to how SAP handles external breakpoints.

**Key insight from Report 2025-12-14-002:**
- Breakpoints ARE stored in `ABDBG_EXTDBPS` table
- Debug activation IS registered in `ICFATTRIB` table
- But breakpoints set via vsp only trigger when:
  1. The SAME user runs the program
  2. The program is triggered via WebSocket RFC (ZADT_VSP)

**Why SAP GUI doesn't hit the breakpoints:**

| Factor | vsp Breakpoint | SAP GUI Execution |
|--------|---------------|-------------------|
| User | Set for user X | Running as user X ✅ |
| Mode | User mode | Different session context |
| Session | HTTP stateless | SAP GUI has own debugger |
| Trigger | Needs listener active | No listener from vsp |

The breakpoint is "external" but SAP GUI starts its OWN debugging session which doesn't share the same context.

### How Debugging is Designed to Work

The vsp debugger is designed for **end-to-end AI-controlled debugging**:

```
1. SetBreakpoint (via WebSocket) → SAP stores breakpoint
2. DebuggerListen (blocks waiting) → Listener ready
3. CallRFC/RunReport (via WebSocket) → Triggers execution
4. SAP hits breakpoint → DebuggerAttach returns
5. GetVariables/Step/etc → Inspect and control
```

When running from SAP GUI:
- SAP GUI has its OWN debugger
- It doesn't see external breakpoints by default
- User would need to enable "external debugging" in SAP GUI settings

### Potential Solutions

#### Option A: Document the Workflow (Immediate)
Clarify that vsp debugging requires:
1. Setting breakpoint via vsp
2. Triggering execution via vsp (RunUnitTests, CallRFC, RunReport)
3. NOT from SAP GUI or other tools

#### Option B: Enable SAP GUI Detection (Medium Effort)
The listener CAN catch SAP GUI executions if:
- User enables SM50 → Debugging → External Breakpoints
- Or: SE80 → Utilities → Settings → Debugging → Allow external debugging

Add documentation explaining how to enable this.

#### Option C: Terminal Mode Breakpoints (Experimental)
Instead of user-mode breakpoints, we could try terminal-mode:
- Set `DebuggingMode: terminal` instead of `user`
- Provide a unique terminal ID that SAP GUI also uses
- **Caveat**: Unlikely to work with GUI as they use different terminals

### Recommendation

1. **Respond to user** explaining the workflow requirement
2. **Add FAQ section** in README about debugging lifecycle
3. **Consider adding** SAP GUI debugging instructions

---

## Reflection

### Significance

This is **fantastic news** - the first external user! The issues reveal:

1. **Real-world usage patterns** differ from our test environment
   - We use HTTP (no self-signed cert issues)
   - We always trigger via WebSocket (no GUI interaction)

2. **Documentation gaps** exist
   - The debugging workflow isn't clearly explained
   - The `--insecure` flag limitation wasn't documented

3. **Opportunity for improvement**
   - Issue #1 is a clear bug with an easy fix
   - Issue #2 needs documentation, not necessarily code changes

### What This Tells Us

The user:
- Successfully installed ZADT_VSP (via abapGit from /src)
- Got WebSocket working ("works fine")
- Tried to use it for real debugging (ALV report)
- Has HTTPS SAP system with self-signed certificate

This is an **advanced user** who:
- Knows abapGit
- Understands WebSocket/APC concepts
- Has development access to SAP
- Is trying real-world debugging scenarios

### Action Items

| Priority | Action | Effort |
|----------|--------|--------|
| **P0** | Fix WebSocket TLS bug (#1) | 15 min |
| **P1** | Respond to both issues | 10 min |
| **P1** | Add debugging workflow FAQ | 30 min |
| **P2** | Add SAP GUI debugging docs | 1 hour |

### Positive Takeaways

1. **The product works** - User got it installed and WebSocket connected
2. **Clear use case** - Real debugging of ALV reports
3. **Engaged community** - Filed detailed, actionable issues
4. **Quick wins available** - Issue #1 is 15-minute fix

---

## Proposed Response to Issues

### Issue #1 Response (Self-signed certificate)

```markdown
Thanks for reporting this @marcellourbani!

You found a bug - the `--insecure` flag works for HTTP requests but NOT for WebSocket connections. The WebSocket dialer was missing the TLS configuration.

I'll have a fix in the next release (v2.19.1). In the meantime, workarounds:
1. Add the SAP cert to your system's trusted certificates
2. Use HTTP instead of HTTPS if possible for development

Fix coming shortly!
```

### Issue #2 Response (GUI debugger)

```markdown
Thanks for the detailed report @marcellourbani!

This is working as designed, but I realize the documentation doesn't explain the workflow clearly.

**vsp debugging is designed for end-to-end AI-controlled execution:**
1. `SetBreakpoint` → Stores breakpoint
2. `DebuggerListen` → Start waiting for debuggee
3. `CallRFC` or `RunReport` or `RunUnitTests` → Trigger execution via WebSocket
4. Debugger catches the breakpoint → `DebuggerAttach` returns
5. `GetVariables`, `Step`, etc → Inspect and control

When you run from SAP GUI, it uses a different debugging session that doesn't share our external breakpoints.

**If you want SAP GUI to hit the breakpoints:**
1. In SAP GUI: SM50 → Debugging → External Breakpoints → Enable
2. Or: SE80 → Utilities → Settings → ABAP Editor → Debugging → Allow external debugging

But the recommended flow is to trigger execution via vsp tools:
- `RunReport(report="YOUR_ALV_REPORT")` - For reports with selection screen
- `RunUnitTests(url=...)` - For test methods
- `CallRFC(function="...")` - For function modules

I'll add a FAQ section to the README to clarify this workflow. Would the SAP GUI integration be valuable for your use case?
```

---

## Files Modified by This Analysis

None yet - this is a research/documentation report.

## Next Steps

1. Fix Issue #1 (WebSocket TLS) - create PR
2. Post responses to GitHub issues
3. Update README with debugging FAQ
4. Tag v2.19.1 with the fix
