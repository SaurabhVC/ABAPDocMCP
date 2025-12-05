// Package adt provides AMDP debug session management via goroutines and channels.
//
// This enables persistent AMDP debug sessions within the stateless MCP architecture.
// A dedicated "Session Manager" goroutine holds the HTTP session cookies while
// MCP tool handlers communicate with it via typed channels.
package adt

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AMDPCommandType represents the type of debug command
type AMDPCommandType int

const (
	// AMDPCmdStart initiates a debug session
	AMDPCmdStart AMDPCommandType = iota
	// AMDPCmdStop terminates the debug session
	AMDPCmdStop
	// AMDPCmdStep performs a step operation
	AMDPCmdStep
	// AMDPCmdGetStatus retrieves session status
	AMDPCmdGetStatus
	// AMDPCmdGetVariables retrieves variable values
	AMDPCmdGetVariables
	// AMDPCmdSetBreakpoint sets a breakpoint
	AMDPCmdSetBreakpoint
	// AMDPCmdGetBreakpoints lists breakpoints
	AMDPCmdGetBreakpoints
)

// AMDPCommand represents a debug command sent to the session manager
type AMDPCommand struct {
	Type     AMDPCommandType
	Args     map[string]interface{}
	Response chan AMDPResponse
}

// AMDPResponse is the result of a debug command
type AMDPResponse struct {
	Success bool
	Data    interface{}
	Error   error
}

// AMDPSessionState represents the state of an AMDP debug session
type AMDPSessionState struct {
	SessionID   string `json:"sessionId"`
	MainID      string `json:"mainId"`
	ObjectURI   string `json:"objectUri"`
	Status      string `json:"status"` // "running", "stopped", "breakpoint"
	CurrentLine int    `json:"currentLine,omitempty"`
	CurrentProc string `json:"currentProc,omitempty"`
}

// AMDPSessionManager manages a persistent AMDP debug session
type AMDPSessionManager struct {
	mu         sync.RWMutex
	running    bool
	state      AMDPSessionState
	cmdChannel chan AMDPCommand
	httpClient *http.Client
	baseURL    string
	client     string // SAP client
	csrfToken  string
	cancel     context.CancelFunc
}

// NewAMDPSessionManager creates a new AMDP session manager
func NewAMDPSessionManager(baseURL, client string, insecure bool) *AMDPSessionManager {
	// Create HTTP client with cookie jar
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
	}

	return &AMDPSessionManager{
		baseURL: baseURL,
		client:  client,
		httpClient: &http.Client{
			Jar:       jar,
			Transport: transport,
			Timeout:   60 * time.Second,
		},
	}
}

// IsRunning returns whether a debug session is active
func (m *AMDPSessionManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// State returns the current session state
func (m *AMDPSessionManager) State() AMDPSessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Start initializes and starts a new AMDP debug session
func (m *AMDPSessionManager) Start(ctx context.Context, objectURI, user, password string) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("AMDP session already active")
	}
	m.running = true
	m.cmdChannel = make(chan AMDPCommand, 10)
	m.state = AMDPSessionState{
		ObjectURI: objectURI,
		Status:    "starting",
	}
	m.mu.Unlock()

	// Create context with cancellation
	sessionCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// Fetch CSRF token first
	if err := m.fetchCSRFToken(sessionCtx, user, password); err != nil {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return fmt.Errorf("failed to fetch CSRF token: %w", err)
	}

	// Start the debug session via ADT API
	if err := m.initiateSession(sessionCtx, objectURI, user, password); err != nil {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return fmt.Errorf("failed to start AMDP session: %w", err)
	}

	// Start command processor goroutine
	go m.processCommands(sessionCtx)

	return nil
}

// Stop terminates the debug session
func (m *AMDPSessionManager) Stop() error {
	m.mu.RLock()
	if !m.running {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	if m.cancel != nil {
		m.cancel()
	}
	return nil
}

// SendCommand sends a command to the session manager and waits for response
func (m *AMDPSessionManager) SendCommand(cmdType AMDPCommandType, args map[string]interface{}) (AMDPResponse, error) {
	m.mu.RLock()
	if !m.running {
		m.mu.RUnlock()
		return AMDPResponse{}, fmt.Errorf("AMDP session not running")
	}
	cmdChan := m.cmdChannel
	m.mu.RUnlock()

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
	case <-time.After(5 * time.Second):
		return AMDPResponse{}, fmt.Errorf("command channel timeout")
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(60 * time.Second):
		return AMDPResponse{}, fmt.Errorf("command response timeout")
	}
}

// processCommands is the main goroutine loop that handles debug commands
func (m *AMDPSessionManager) processCommands(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash
			fmt.Printf("AMDP session panic: %v\n", r)
		}
		m.cleanup(ctx)
	}()

	// Keepalive ticker to prevent session timeout (every 30 seconds)
	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, clean up
			return

		case <-keepalive.C:
			// Send keepalive to prevent session timeout
			m.sendKeepalive(ctx)

		case cmd, ok := <-m.cmdChannel:
			if !ok {
				return
			}

			// Process command using the persistent session
			resp := m.handleCommand(ctx, cmd)

			// Send response
			select {
			case cmd.Response <- resp:
			default:
				// Response channel full or closed, skip
			}

			// If stop command, exit the goroutine
			if cmd.Type == AMDPCmdStop {
				return
			}
		}
	}
}

// handleCommand processes a single debug command
func (m *AMDPSessionManager) handleCommand(ctx context.Context, cmd AMDPCommand) AMDPResponse {
	switch cmd.Type {
	case AMDPCmdStep:
		stepType, _ := cmd.Args["step_type"].(string)
		result, err := m.step(ctx, stepType)
		return AMDPResponse{Success: err == nil, Data: result, Error: err}

	case AMDPCmdGetStatus:
		status, err := m.getStatus(ctx)
		return AMDPResponse{Success: err == nil, Data: status, Error: err}

	case AMDPCmdGetVariables:
		vars, err := m.getVariables(ctx)
		return AMDPResponse{Success: err == nil, Data: vars, Error: err}

	case AMDPCmdGetBreakpoints:
		bps, err := m.getBreakpoints(ctx)
		return AMDPResponse{Success: err == nil, Data: bps, Error: err}

	case AMDPCmdSetBreakpoint:
		procName, _ := cmd.Args["proc_name"].(string)
		line, _ := cmd.Args["line"].(int)
		err := m.setBreakpoint(ctx, procName, line)
		return AMDPResponse{Success: err == nil, Error: err}

	case AMDPCmdStop:
		err := m.stopSession(ctx)
		return AMDPResponse{Success: err == nil, Error: err}

	default:
		return AMDPResponse{Error: fmt.Errorf("unknown command: %d", cmd.Type)}
	}
}

// cleanup releases resources when the session ends
func (m *AMDPSessionManager) cleanup(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try to stop session on server
	if m.state.MainID != "" {
		m.hardStop(ctx)
	}

	m.running = false
	m.state = AMDPSessionState{}
	if m.cmdChannel != nil {
		close(m.cmdChannel)
		m.cmdChannel = nil
	}
}

// fetchCSRFToken retrieves a CSRF token for subsequent requests
func (m *AMDPSessionManager) fetchCSRFToken(ctx context.Context, user, password string) error {
	u := fmt.Sprintf("%s/sap/bc/adt/discovery?sap-client=%s", m.baseURL, m.client)

	req, err := http.NewRequestWithContext(ctx, "HEAD", u, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(user, password)
	req.Header.Set("X-CSRF-Token", "fetch")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	m.csrfToken = resp.Header.Get("X-CSRF-Token")
	if m.csrfToken == "" || m.csrfToken == "unsafe" {
		return fmt.Errorf("failed to get CSRF token")
	}

	return nil
}

// initiateSession starts an AMDP debug session on the server
func (m *AMDPSessionManager) initiateSession(ctx context.Context, objectURI, user, password string) error {
	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions?sap-client=%s", m.baseURL, m.client)

	// Build session start XML
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<amdp:startConfiguration xmlns:amdp="http://www.sap.com/adt/debugger/amdp">
  <amdp:objectUri>%s</amdp:objectUri>
  <amdp:user>%s</amdp:user>
  <amdp:terminateExisting>true</amdp:terminateExisting>
</amdp:startConfiguration>`, objectURI, user)

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(body))
	if err != nil {
		return err
	}

	req.SetBasicAuth(user, password)
	req.Header.Set("X-CSRF-Token", m.csrfToken)
	req.Header.Set("Content-Type", "application/vnd.sap.adt.debugger.amdp.startconfiguration.v1+xml")
	req.Header.Set("Accept", "application/vnd.sap.adt.debugger.amdp.session.v1+xml")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to start session: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response to get session and main IDs
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse the session response
	var sessionResp struct {
		XMLName   xml.Name `xml:"session"`
		SessionID string   `xml:"sessionId"`
		MainID    string   `xml:"mainId"`
	}
	if err := xml.Unmarshal(bodyBytes, &sessionResp); err != nil {
		// Try to extract from response anyway
		return fmt.Errorf("session started but failed to parse response: %s", string(bodyBytes))
	}

	m.mu.Lock()
	m.state.SessionID = sessionResp.SessionID
	m.state.MainID = sessionResp.MainID
	m.state.Status = "running"
	m.mu.Unlock()

	return nil
}

// step performs a step operation in the debug session
func (m *AMDPSessionManager) step(ctx context.Context, stepType string) (map[string]interface{}, error) {
	m.mu.RLock()
	mainID := m.state.MainID
	m.mu.RUnlock()

	if mainID == "" {
		return nil, fmt.Errorf("no active session")
	}

	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s?sap-client=%s",
		m.baseURL, url.PathEscape(mainID), m.client)

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<amdp:stepConfiguration xmlns:amdp="http://www.sap.com/adt/debugger/amdp">
  <amdp:stepType>%s</amdp:stepType>
</amdp:stepConfiguration>`, stepType)

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-CSRF-Token", m.csrfToken)
	req.Header.Set("Content-Type", "application/vnd.sap.adt.debugger.amdp.stepconfiguration.v1+xml")
	req.Header.Set("Accept", "application/vnd.sap.adt.debugger.amdp.stepresult.v1+xml")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("step failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse step result
	result := map[string]interface{}{
		"stepType": stepType,
		"response": string(bodyBytes),
	}

	return result, nil
}

// getStatus retrieves the current debug session status
func (m *AMDPSessionManager) getStatus(ctx context.Context) (*AMDPSessionState, error) {
	m.mu.RLock()
	mainID := m.state.MainID
	m.mu.RUnlock()

	if mainID == "" {
		return nil, fmt.Errorf("no active session")
	}

	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s?sap-client=%s",
		m.baseURL, url.PathEscape(mainID), m.client)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.sap.adt.debugger.amdp.session.v1+xml")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get status failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	// Return current state
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()

	return &state, nil
}

// getVariables retrieves variable values from the debug session
func (m *AMDPSessionManager) getVariables(ctx context.Context) ([]map[string]interface{}, error) {
	m.mu.RLock()
	mainID := m.state.MainID
	m.mu.RUnlock()

	if mainID == "" {
		return nil, fmt.Errorf("no active session")
	}

	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s/variables?sap-client=%s",
		m.baseURL, url.PathEscape(mainID), m.client)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.sap.adt.debugger.amdp.variables.v1+xml")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get variables failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	// Return raw response for now
	return []map[string]interface{}{
		{"response": string(bodyBytes)},
	}, nil
}

// getBreakpoints retrieves the list of breakpoints
func (m *AMDPSessionManager) getBreakpoints(ctx context.Context) ([]map[string]interface{}, error) {
	m.mu.RLock()
	mainID := m.state.MainID
	m.mu.RUnlock()

	if mainID == "" {
		return nil, fmt.Errorf("no active session")
	}

	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s/breakpoints?sap-client=%s",
		m.baseURL, url.PathEscape(mainID), m.client)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.sap.adt.debugger.amdp.breakpoints.v1+xml")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get breakpoints failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return []map[string]interface{}{
		{"response": string(bodyBytes)},
	}, nil
}

// setBreakpoint sets a breakpoint at the specified location
func (m *AMDPSessionManager) setBreakpoint(ctx context.Context, procName string, line int) error {
	m.mu.RLock()
	mainID := m.state.MainID
	m.mu.RUnlock()

	if mainID == "" {
		return fmt.Errorf("no active session")
	}

	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s/breakpoints?sap-client=%s",
		m.baseURL, url.PathEscape(mainID), m.client)

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<amdp:breakpoint xmlns:amdp="http://www.sap.com/adt/debugger/amdp">
  <amdp:procName>%s</amdp:procName>
  <amdp:line>%d</amdp:line>
</amdp:breakpoint>`, procName, line)

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("X-CSRF-Token", m.csrfToken)
	req.Header.Set("Content-Type", "application/vnd.sap.adt.debugger.amdp.breakpoint.v1+xml")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set breakpoint failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// stopSession gracefully stops the debug session
func (m *AMDPSessionManager) stopSession(ctx context.Context) error {
	m.mu.RLock()
	mainID := m.state.MainID
	m.mu.RUnlock()

	if mainID == "" {
		return nil // Already stopped
	}

	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s?sap-client=%s",
		m.baseURL, url.PathEscape(mainID), m.client)

	req, err := http.NewRequestWithContext(ctx, "DELETE", u, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-CSRF-Token", m.csrfToken)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept 200, 204, or 404 (already stopped)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop session failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// hardStop forcefully terminates the session
func (m *AMDPSessionManager) hardStop(ctx context.Context) {
	mainID := m.state.MainID
	if mainID == "" {
		return
	}

	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s?sap-client=%s&hard_stop=true",
		m.baseURL, url.PathEscape(mainID), m.client)

	req, _ := http.NewRequestWithContext(ctx, "DELETE", u, nil)
	if req != nil {
		req.Header.Set("X-CSRF-Token", m.csrfToken)
		m.httpClient.Do(req)
	}
}

// sendKeepalive sends a keepalive request to prevent session timeout
func (m *AMDPSessionManager) sendKeepalive(ctx context.Context) {
	m.mu.RLock()
	mainID := m.state.MainID
	m.mu.RUnlock()

	if mainID == "" {
		return
	}

	// Simple GET request to keep session alive
	u := fmt.Sprintf("%s/sap/bc/adt/runtime/debugger/amdp/sessions/%s?sap-client=%s",
		m.baseURL, url.PathEscape(mainID), m.client)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
