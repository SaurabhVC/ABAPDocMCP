package adt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// AMDPWebSocketClient manages AMDP debugging via WebSocket (ZADT_VSP).
// This replaces the HTTP-based AMDPSessionManager for more reliable debugging.
type AMDPWebSocketClient struct {
	baseURL   string
	client    string
	user      string
	password  string
	insecure  bool

	conn      *websocket.Conn
	sessionID string
	mu        sync.RWMutex

	// Request/response handling
	msgID     atomic.Int64
	pending   map[string]chan *WSResponse
	pendingMu sync.Mutex

	// Welcome signal
	welcomeCh chan struct{}

	// State
	contextID string
	isActive  bool

	// Event channel for async events (breakpoint hits, etc.)
	Events chan *AMDPEvent
}

// WSMessage is the WebSocket message format for ZADT_VSP.
type WSMessage struct {
	ID      string                 `json:"id"`
	Domain  string                 `json:"domain"`
	Action  string                 `json:"action"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Timeout int                    `json:"timeout,omitempty"`
}

// WSResponse is the WebSocket response format from ZADT_VSP.
type WSResponse struct {
	ID      string          `json:"id"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *WSError        `json:"error,omitempty"`
}

// WSError represents an error from ZADT_VSP.
type WSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AMDPEvent represents an async event from AMDP debugger.
type AMDPEvent struct {
	Kind       string                 `json:"kind"`
	ContextID  string                 `json:"context_id,omitempty"`
	Position   *AMDPPosition          `json:"position,omitempty"`
	Variables  []AMDPVariable         `json:"variables,omitempty"`
	StackDepth int                    `json:"stack_depth,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// NewAMDPWebSocketClient creates a new WebSocket-based AMDP client.
func NewAMDPWebSocketClient(baseURL, client, user, password string, insecure bool) *AMDPWebSocketClient {
	return &AMDPWebSocketClient{
		baseURL:   baseURL,
		client:    client,
		user:      user,
		password:  password,
		insecure:  insecure,
		pending:   make(map[string]chan *WSResponse),
		welcomeCh: make(chan struct{}, 1),
		Events:    make(chan *AMDPEvent, 10),
	}
}

// Connect establishes WebSocket connection to ZADT_VSP.
func (c *AMDPWebSocketClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.conn != nil {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}

	// Build WebSocket URL
	// Convert http://host:port to ws://host:port/sap/bc/apc/sap/zadt_vsp
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/sap/bc/apc/sap/zadt_vsp?sap-client=%s", scheme, u.Host, c.client)

	// Create dialer with auth and TLS config
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.insecure,
		},
	}

	// Add basic auth header
	header := http.Header{}
	header.Set("Authorization", basicAuth(c.user, c.password))

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}

	c.conn = conn
	c.mu.Unlock()

	// Start message reader goroutine
	go c.readMessages()

	// Wait for welcome message
	select {
	case <-c.welcomeCh:
		// Welcome received successfully
		return nil
	case <-time.After(5 * time.Second):
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
		return fmt.Errorf("timeout waiting for welcome message")
	case <-ctx.Done():
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
		return ctx.Err()
	}
}

// readMessages reads messages from WebSocket and routes them.
func (c *AMDPWebSocketClient) readMessages() {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			// Connection closed
			c.mu.Lock()
			c.conn = nil
			c.isActive = false
			c.mu.Unlock()
			return
		}

		var resp WSResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}

		// Check if this is a response to a pending request
		c.pendingMu.Lock()
		if ch, ok := c.pending[resp.ID]; ok {
			ch <- &resp
			delete(c.pending, resp.ID)
			c.pendingMu.Unlock()
			continue
		}
		c.pendingMu.Unlock()

		// Otherwise it's an async event (e.g., welcome, breakpoint hit)
		if resp.ID == "welcome" {
			// Parse welcome data
			var welcomeData struct {
				Session string   `json:"session"`
				Version string   `json:"version"`
				Domains []string `json:"domains"`
			}
			if err := json.Unmarshal(resp.Data, &welcomeData); err == nil {
				c.mu.Lock()
				c.sessionID = welcomeData.Session
				c.mu.Unlock()
			}
			// Signal that welcome was received
			select {
			case c.welcomeCh <- struct{}{}:
			default:
				// Channel already has signal
			}
		}
	}
}

// sendRequest sends a request and waits for response.
func (c *AMDPWebSocketClient) sendRequest(ctx context.Context, action string, params map[string]interface{}) (*WSResponse, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Generate unique message ID
	id := fmt.Sprintf("amdp_%d", c.msgID.Add(1))

	msg := WSMessage{
		ID:      id,
		Domain:  "amdp",
		Action:  action,
		Params:  params,
		Timeout: 60000,
	}

	// Create response channel
	respCh := make(chan *WSResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	// Send message
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("send failed: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(60 * time.Second):
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("request timeout")
	}
}

// Start starts an AMDP debug session.
func (c *AMDPWebSocketClient) Start(ctx context.Context, cascadeMode string) error {
	if cascadeMode == "" {
		cascadeMode = "FULL"
	}

	params := map[string]interface{}{
		"user":        c.user,
		"cascadeMode": cascadeMode,
	}

	resp, err := c.sendRequest(ctx, "start", params)
	if err != nil {
		return err
	}

	if !resp.Success {
		if resp.Error != nil {
			return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("start failed")
	}

	c.mu.Lock()
	c.isActive = true
	c.mu.Unlock()

	return nil
}

// Stop stops the AMDP debug session.
func (c *AMDPWebSocketClient) Stop(ctx context.Context) error {
	resp, err := c.sendRequest(ctx, "stop", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		if resp.Error != nil {
			return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("stop failed")
	}

	c.mu.Lock()
	c.isActive = false
	c.contextID = ""
	c.mu.Unlock()

	return nil
}

// Resume resumes the debugger and waits for events.
func (c *AMDPWebSocketClient) Resume(ctx context.Context) (*AMDPResumeResult, error) {
	resp, err := c.sendRequest(ctx, "resume", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("resume failed")
	}

	var result AMDPResumeResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parse resume result: %w", err)
	}

	// Update context_id if we got a breakpoint hit
	for _, event := range result.Events {
		if event.Kind == "on_break" && event.ContextID != "" {
			c.mu.Lock()
			c.contextID = event.ContextID
			c.mu.Unlock()
			break
		}
	}

	return &result, nil
}

// AMDPResumeResult contains the result of a resume operation.
type AMDPResumeResult struct {
	Events []AMDPResumeEvent `json:"events"`
}

// AMDPResumeEvent represents an event from resume.
type AMDPResumeEvent struct {
	Kind          string            `json:"kind"`
	ContextID     string            `json:"context_id,omitempty"`
	BPClientID    string            `json:"bp_client_id,omitempty"`
	ABAPPosition  *AMDPABAPPosition `json:"abap_position,omitempty"`
	NativePosition *AMDPNativePosition `json:"native_position,omitempty"`
	VariableCount int               `json:"variable_count,omitempty"`
	StackDepth    int               `json:"stack_depth,omitempty"`
	Aborted       bool              `json:"aborted,omitempty"`
}

// AMDPABAPPosition represents a position in ABAP source.
type AMDPABAPPosition struct {
	Program string `json:"program"`
	Include string `json:"include"`
	Line    int    `json:"line"`
}

// AMDPNativePosition represents a position in SQLScript.
type AMDPNativePosition struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Line   int    `json:"line"`
}

// Step performs a step operation.
func (c *AMDPWebSocketClient) Step(ctx context.Context, stepType string) error {
	if stepType == "" {
		stepType = "over"
	}

	params := map[string]interface{}{
		"type": stepType,
	}

	resp, err := c.sendRequest(ctx, "step", params)
	if err != nil {
		return err
	}

	if !resp.Success {
		if resp.Error != nil {
			return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("step failed")
	}

	return nil
}

// SetBreakpoint sets a breakpoint in AMDP code.
func (c *AMDPWebSocketClient) SetBreakpoint(ctx context.Context, program string, line int) error {
	params := map[string]interface{}{
		"program": program,
		"line":    line,
	}

	resp, err := c.sendRequest(ctx, "setBreakpoint", params)
	if err != nil {
		return err
	}

	if !resp.Success {
		if resp.Error != nil {
			return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("set breakpoint failed")
	}

	return nil
}

// GetBreakpoints returns currently set AMDP breakpoints.
func (c *AMDPWebSocketClient) GetBreakpoints(ctx context.Context) (*AMDPBreakpointsResult, error) {
	resp, err := c.sendRequest(ctx, "getBreakpoints", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("get breakpoints failed")
	}

	var result AMDPBreakpointsResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parse breakpoints result: %w", err)
	}

	return &result, nil
}

// AMDPBreakpointsResult contains breakpoint information.
type AMDPBreakpointsResult struct {
	Breakpoints []AMDPBreakpoint `json:"breakpoints"`
}

// GetVariables returns AMDP session variables.
func (c *AMDPWebSocketClient) GetVariables(ctx context.Context) (*AMDPVariablesResult, error) {
	resp, err := c.sendRequest(ctx, "getVariables", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("get variables failed")
	}

	var result AMDPVariablesResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parse variables result: %w", err)
	}

	return &result, nil
}

// AMDPVariablesResult contains variable information.
type AMDPVariablesResult struct {
	Variables []AMDPVariable `json:"variables"`
}

// GetStatus returns current session status.
func (c *AMDPWebSocketClient) GetStatus(ctx context.Context) (*AMDPStatusResult, error) {
	resp, err := c.sendRequest(ctx, "getStatus", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("get status failed")
	}

	var result AMDPStatusResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parse status result: %w", err)
	}

	return &result, nil
}

// AMDPStatusResult contains session status.
type AMDPStatusResult struct {
	Active    bool   `json:"active"`
	ContextID string `json:"context_id"`
}

// Execute runs an AMDP method within the debug session context.
// This allows breakpoints to be hit since execution is in the same session.
func (c *AMDPWebSocketClient) Execute(ctx context.Context, class, method string, count int) (*AMDPExecuteResult, error) {
	params := map[string]interface{}{
		"class":  class,
		"method": method,
		"count":  count,
	}

	resp, err := c.sendRequest(ctx, "execute", params)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("execute failed")
	}

	var result AMDPExecuteResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parse execute result: %w", err)
	}

	return &result, nil
}

// AMDPExecuteResult contains execution result.
type AMDPExecuteResult struct {
	Status string              `json:"status"`
	Class  string              `json:"class"`
	Method string              `json:"method"`
	Rows   int                 `json:"rows"`
	Data   []AMDPExecuteRow    `json:"data"`
}

// AMDPExecuteRow contains a result row.
type AMDPExecuteRow struct {
	ID     int    `json:"id"`
	Value  string `json:"value"`
	Square int    `json:"square"`
}

// ExecuteAndDebug combines start, breakpoint, execute, and resume in a single call.
// This solves the session blocking issue by running everything in one ABAP request.
// Returns debug events from hitting the breakpoint.
func (c *AMDPWebSocketClient) ExecuteAndDebug(ctx context.Context, class, method string, line, count int, cascadeMode string) (*AMDPExecuteDebugResult, error) {
	if cascadeMode == "" {
		cascadeMode = "FULL"
	}

	params := map[string]interface{}{
		"class":       class,
		"method":      method,
		"line":        line,
		"count":       count,
		"cascadeMode": cascadeMode,
	}

	resp, err := c.sendRequest(ctx, "executeAndDebug", params)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("executeAndDebug failed")
	}

	var result AMDPExecuteDebugResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parse executeAndDebug result: %w", err)
	}

	// Update context_id if we got a breakpoint hit
	for _, event := range result.Events {
		if event.Kind == "on_break" && event.ContextID != "" {
			c.mu.Lock()
			c.contextID = event.ContextID
			c.isActive = true
			c.mu.Unlock()
			break
		}
	}

	return &result, nil
}

// AMDPExecuteDebugResult contains the result of executeAndDebug operation.
type AMDPExecuteDebugResult struct {
	Status         string            `json:"status"`
	Class          string            `json:"class"`
	Method         string            `json:"method"`
	Line           int               `json:"line"`
	ExecutionRows  int               `json:"execution_rows"`
	ExecutionError string            `json:"execution_error,omitempty"`
	Events         []AMDPResumeEvent `json:"events"`
}

// Close closes the WebSocket connection.
func (c *AMDPWebSocketClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.isActive = false
	return nil
}

// IsConnected returns true if WebSocket is connected.
func (c *AMDPWebSocketClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil
}

// IsActive returns true if AMDP session is active.
func (c *AMDPWebSocketClient) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isActive
}

// basicAuth creates basic auth header value.
func basicAuth(user, password string) string {
	auth := user + ":" + password
	return "Basic " + base64Encode([]byte(auth))
}

// base64Encode encodes bytes to base64 string.
func base64Encode(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte
	for i := 0; i < len(data); i += 3 {
		var b uint32
		remaining := len(data) - i
		if remaining >= 3 {
			b = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result, base64Chars[b>>18&0x3F], base64Chars[b>>12&0x3F], base64Chars[b>>6&0x3F], base64Chars[b&0x3F])
		} else if remaining == 2 {
			b = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result, base64Chars[b>>18&0x3F], base64Chars[b>>12&0x3F], base64Chars[b>>6&0x3F], '=')
		} else {
			b = uint32(data[i]) << 16
			result = append(result, base64Chars[b>>18&0x3F], base64Chars[b>>12&0x3F], '=', '=')
		}
	}
	return string(result)
}
