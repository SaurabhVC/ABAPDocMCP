# AMDP Debug Session Manager: Goroutine + Channel Architecture

**Date:** 2025-12-05
**Report ID:** 024
**Subject:** Persistent AMDP Debug Sessions via Go Concurrency Patterns
**Related:** Report 019 (AMDP Session Architecture)

---

## Executive Summary

This design proposes using Go's native concurrency primitives (goroutines and channels) to maintain persistent AMDP debug sessions within the stateless MCP architecture. A dedicated "Session Manager" goroutine holds the HTTP session cookies while MCP tool handlers communicate with it via typed channels.

---

## The Problem (Recap)

AMDP debugging requires:
1. **Session binding**: Debug session is tied to HTTP cookies (SAP_SESSIONID_*)
2. **Persistent connection**: Same session must be used for start → step → inspect → stop
3. **MCP statelessness**: Each MCP tool call is conceptually independent

Previous solutions considered (Report 019):
- Expose raw cookies to AI (security risk)
- External session store (complexity)
- Sticky session middleware (external dependency)

**New solution**: Goroutine + Channels (internal to vsp, no external dependencies)

---

## Architecture Design

### Core Components

```go
// pkg/adt/amdp_session.go

// AMDPCommand represents a debug command sent to the session manager
type AMDPCommand struct {
    Type     AMDPCommandType
    Args     map[string]interface{}
    Response chan AMDPResponse
}

type AMDPCommandType int

const (
    AMDPCmdStart AMDPCommandType = iota
    AMDPCmdStop
    AMDPCmdStep
    AMDPCmdGetStatus
    AMDPCmdGetVariables
    AMDPCmdSetBreakpoint
    AMDPCmdClearBreakpoint
)

// AMDPResponse is the result of a debug command
type AMDPResponse struct {
    Success bool
    Data    interface{}
    Error   error
}

// AMDPSessionManager manages a persistent AMDP debug session
type AMDPSessionManager struct {
    mu          sync.Mutex
    running     bool
    sessionID   string
    mainID      string
    cmdChannel  chan AMDPCommand
    httpClient  *http.Client  // Holds session cookies
    baseURL     string
    cancel      context.CancelFunc
}
```

### Session Manager Goroutine

```go
// Start creates and starts a new AMDP session manager
func (m *AMDPSessionManager) Start(ctx context.Context, objectURI string) error {
    m.mu.Lock()
    if m.running {
        m.mu.Unlock()
        return fmt.Errorf("AMDP session already active")
    }
    m.running = true
    m.cmdChannel = make(chan AMDPCommand, 10)
    m.mu.Unlock()

    // Create context with cancellation
    ctx, m.cancel = context.WithCancel(ctx)

    // Create dedicated HTTP client for this session
    m.httpClient = &http.Client{
        Jar: &sessionCookieJar{}, // Custom jar that persists SAP session
    }

    // Start the debug session
    sessionID, mainID, err := m.initiateSession(ctx, objectURI)
    if err != nil {
        m.running = false
        return err
    }
    m.sessionID = sessionID
    m.mainID = mainID

    // Start command processor goroutine
    go m.processCommands(ctx)

    return nil
}

// processCommands is the main goroutine loop
func (m *AMDPSessionManager) processCommands(ctx context.Context) {
    defer func() {
        m.mu.Lock()
        m.running = false
        close(m.cmdChannel)
        m.mu.Unlock()
    }()

    for {
        select {
        case <-ctx.Done():
            // Context cancelled, clean up
            m.cleanup()
            return

        case cmd, ok := <-m.cmdChannel:
            if !ok {
                return
            }

            // Process command using the persistent session
            resp := m.handleCommand(ctx, cmd)
            cmd.Response <- resp
            close(cmd.Response)

            // If stop command, exit the goroutine
            if cmd.Type == AMDPCmdStop {
                m.cancel()
                return
            }
        }
    }
}

// handleCommand processes a single debug command
func (m *AMDPSessionManager) handleCommand(ctx context.Context, cmd AMDPCommand) AMDPResponse {
    switch cmd.Type {
    case AMDPCmdStep:
        stepType := cmd.Args["step_type"].(string)
        result, err := m.step(ctx, stepType)
        return AMDPResponse{Success: err == nil, Data: result, Error: err}

    case AMDPCmdGetStatus:
        status, err := m.getStatus(ctx)
        return AMDPResponse{Success: err == nil, Data: status, Error: err}

    case AMDPCmdGetVariables:
        vars, err := m.getVariables(ctx)
        return AMDPResponse{Success: err == nil, Data: vars, Error: err}

    case AMDPCmdStop:
        err := m.stopSession(ctx)
        return AMDPResponse{Success: err == nil, Error: err}

    default:
        return AMDPResponse{Error: fmt.Errorf("unknown command: %d", cmd.Type)}
    }
}
```

### MCP Tool Integration

```go
// internal/mcp/server.go

type Server struct {
    client       *adt.Client
    amdpSession  *adt.AMDPSessionManager  // Add session manager
    // ...
}

// handleAMDPDebuggerStart starts a persistent AMDP debug session
func (s *Server) handleAMDPDebuggerStart(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
    objectURI, _ := getString(args, "object_uri")

    // Check if session already active
    if s.amdpSession != nil && s.amdpSession.IsRunning() {
        return mcp.NewToolResultError("AMDP session already active. Use AMDPDebuggerStop first."), nil
    }

    // Create new session manager
    s.amdpSession = adt.NewAMDPSessionManager(s.client.BaseURL(), s.client.HTTPConfig())

    if err := s.amdpSession.Start(ctx, objectURI); err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Failed to start AMDP session: %v", err)), nil
    }

    return mcp.NewToolResultText(fmt.Sprintf(
        "AMDP debug session started.\nSession ID: %s\nMain ID: %s\nUse AMDPDebuggerStep, AMDPDebuggerGetStatus, etc. to interact.",
        s.amdpSession.SessionID(),
        s.amdpSession.MainID(),
    )), nil
}

// handleAMDPDebuggerStep sends a step command to the active session
func (s *Server) handleAMDPDebuggerStep(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
    if s.amdpSession == nil || !s.amdpSession.IsRunning() {
        return mcp.NewToolResultError("No active AMDP session. Use AMDPDebuggerStart first."), nil
    }

    stepType, _ := getString(args, "step_type")

    // Send command via channel and wait for response
    resp, err := s.amdpSession.SendCommand(AMDPCmdStep, map[string]interface{}{
        "step_type": stepType,
    })
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Step failed: %v", err)), nil
    }

    return mcp.NewToolResultText(formatAMDPStepResult(resp.Data)), nil
}

// handleAMDPDebuggerStop terminates the debug session
func (s *Server) handleAMDPDebuggerStop(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
    if s.amdpSession == nil {
        return mcp.NewToolResultText("No AMDP session active."), nil
    }

    resp, err := s.amdpSession.SendCommand(AMDPCmdStop, nil)
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Stop failed: %v", err)), nil
    }

    s.amdpSession = nil
    return mcp.NewToolResultText("AMDP debug session stopped."), nil
}
```

### Channel Communication Helper

```go
// SendCommand sends a command to the session manager and waits for response
func (m *AMDPSessionManager) SendCommand(cmdType AMDPCommandType, args map[string]interface{}) (AMDPResponse, error) {
    m.mu.Lock()
    if !m.running {
        m.mu.Unlock()
        return AMDPResponse{}, fmt.Errorf("session not running")
    }
    cmdChan := m.cmdChannel
    m.mu.Unlock()

    // Create response channel for this specific command
    respChan := make(chan AMDPResponse, 1)

    // Send command
    select {
    case cmdChan <- AMDPCommand{
        Type:     cmdType,
        Args:     args,
        Response: respChan,
    }:
        // Command sent successfully
    default:
        return AMDPResponse{}, fmt.Errorf("command channel full")
    }

    // Wait for response with timeout
    select {
    case resp := <-respChan:
        return resp, nil
    case <-time.After(30 * time.Second):
        return AMDPResponse{}, fmt.Errorf("command timeout")
    }
}
```

---

## Sequence Diagram

```
AI/Claude          MCP Server           Session Manager        SAP System
    │                  │                 (goroutine)              │
    │                  │                      │                   │
    │ AMDPDebuggerStart│                      │                   │
    │─────────────────►│                      │                   │
    │                  │  NewSessionManager() │                   │
    │                  │─────────────────────►│                   │
    │                  │                      │                   │
    │                  │  Start(objectURI)    │                   │
    │                  │─────────────────────►│ POST /start       │
    │                  │                      │──────────────────►│
    │                  │                      │◄──────────────────│
    │                  │                      │ (stores cookies)  │
    │                  │◄─────────────────────│                   │
    │◄─────────────────│ Session started      │                   │
    │                  │                      │                   │
    │ AMDPDebuggerStep │                      │                   │
    │─────────────────►│                      │                   │
    │                  │ SendCommand(Step)    │                   │
    │                  │═════════════════════►│ (via channel)     │
    │                  │                      │                   │
    │                  │                      │ POST /step        │
    │                  │                      │──────────────────►│
    │                  │                      │◄──────────────────│
    │                  │◄═════════════════════│ (via channel)     │
    │◄─────────────────│ Step result          │                   │
    │                  │                      │                   │
    │ AMDPDebuggerStop │                      │                   │
    │─────────────────►│                      │                   │
    │                  │ SendCommand(Stop)    │                   │
    │                  │═════════════════════►│                   │
    │                  │                      │ DELETE /session   │
    │                  │                      │──────────────────►│
    │                  │                      │ (goroutine exits) │
    │◄─────────────────│ Session stopped      │                   │
    │                  │                      X                   │
```

---

## Benefits

| Aspect | Benefit |
|--------|---------|
| **No External Dependencies** | Uses Go's built-in concurrency primitives |
| **Session Isolation** | Each AMDP session has its own HTTP client + cookies |
| **Thread Safety** | Channels provide safe communication between MCP handlers and session |
| **Clean Lifecycle** | Goroutine auto-cleans on context cancellation or stop command |
| **Timeout Handling** | Built-in timeout on command responses |
| **Single Binary** | No separate processes or external state stores needed |

---

## Edge Cases & Handling

### 1. Multiple Sessions
```go
// Option A: Single session per server (simpler)
if s.amdpSession != nil && s.amdpSession.IsRunning() {
    return error("session already active")
}

// Option B: Named sessions (more flexible)
type Server struct {
    amdpSessions map[string]*AMDPSessionManager
}
```

### 2. Session Timeout
```go
// Keepalive ticker in goroutine
func (m *AMDPSessionManager) processCommands(ctx context.Context) {
    keepalive := time.NewTicker(30 * time.Second)
    defer keepalive.Stop()

    for {
        select {
        case <-keepalive.C:
            // Send keepalive to prevent session timeout
            m.sendKeepalive(ctx)
        case cmd := <-m.cmdChannel:
            // ... handle command
        }
    }
}
```

### 3. Server Shutdown
```go
func (s *Server) Shutdown() {
    if s.amdpSession != nil {
        s.amdpSession.Stop()
    }
}
```

### 4. Panic Recovery
```go
func (m *AMDPSessionManager) processCommands(ctx context.Context) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("AMDP session panic: %v", r)
        }
        m.cleanup()
    }()
    // ...
}
```

---

## Implementation Plan

### Phase 1: Core Infrastructure (2-3 hours)
1. Create `pkg/adt/amdp_session.go` with:
   - AMDPCommand, AMDPResponse types
   - AMDPSessionManager struct
   - Start(), Stop(), SendCommand() methods
   - processCommands() goroutine

### Phase 2: HTTP Session Handling (1-2 hours)
1. Custom cookie jar that preserves SAP session cookies
2. HTTP methods that use the persistent client:
   - initiateSession()
   - step()
   - getStatus()
   - getVariables()
   - stopSession()

### Phase 3: MCP Integration (1-2 hours)
1. Add amdpSession field to Server struct
2. Refactor existing AMDP handlers to use SendCommand()
3. Add session state checks

### Phase 4: Testing (2-3 hours)
1. Unit tests with mock HTTP
2. Integration tests with real SAP system
3. Timeout and error condition tests

---

## Comparison with Alternatives

| Solution | Complexity | Dependencies | Security | Status |
|----------|------------|--------------|----------|--------|
| Goroutine + Channels | Low | None | High | **Proposed** |
| External Cookie Store | Medium | Redis/file | Medium | Considered |
| Sticky Middleware | High | Nginx/HAProxy | Medium | Considered |
| Expose Cookies to AI | Low | None | **Low** | Rejected |
| WebSocket Bridge | High | WS library | Medium | Considered |

---

## Implementation Status

**✅ IMPLEMENTED** - The goroutine + channel architecture has been implemented:

### Files Created/Modified

| File | Changes |
|------|---------|
| `pkg/adt/amdp_session.go` | **NEW** - AMDPSessionManager with goroutine + channels (~450 LOC) |
| `internal/mcp/server.go` | Updated AMDP handlers to use session manager |

### Key Components

1. **AMDPSessionManager** (`pkg/adt/amdp_session.go`)
   - `NewAMDPSessionManager()` - Creates manager with HTTP client
   - `Start()` - Initiates session and spawns goroutine
   - `Stop()` - Signals goroutine to terminate
   - `SendCommand()` - Type-safe channel communication
   - `processCommands()` - Main goroutine loop with keepalive

2. **MCP Handlers** (updated in `server.go`)
   - `handleAMDPDebuggerStart` - Creates session manager, spawns goroutine
   - `handleAMDPDebuggerResume` - Gets status via channel
   - `handleAMDPDebuggerStep` - Sends step command via channel
   - `handleAMDPGetVariables` - Gets variables via channel
   - `handleAMDPDebuggerStop` - Signals stop, cleans up

---

## Conclusion

The goroutine + channel architecture is the ideal solution for AMDP session persistence:

1. **Native Go** - Uses idiomatic Go patterns
2. **Zero dependencies** - No external services needed
3. **Single binary** - Maintains vsp's deployment simplicity
4. **Secure** - Session cookies never exposed to AI
5. **Clean abstraction** - MCP handlers remain stateless

**Status**: Implemented and tested. Ready for integration testing with real SAP system.
