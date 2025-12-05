package adt

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
)

// --- Transport Types ---

// TransportRequest represents a transport request (workbench or customizing)
type TransportRequest struct {
	Number      string          `json:"number"`
	Owner       string          `json:"owner"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Target      string          `json:"target,omitempty"`
	Type        string          `json:"type"` // workbench or customizing
	Tasks       []TransportTask `json:"tasks,omitempty"`
}

// TransportTask represents a task within a transport request
type TransportTask struct {
	Number      string            `json:"number"`
	Owner       string            `json:"owner"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Objects     []TransportObject `json:"objects,omitempty"`
}

// TransportObject represents an object in a transport task
type TransportObject struct {
	PGMID   string `json:"pgmid"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	ObjInfo string `json:"objInfo,omitempty"`
}

// UserTransports represents transport requests for a user
type UserTransports struct {
	Workbench   []TransportRequest `json:"workbench"`
	Customizing []TransportRequest `json:"customizing"`
}

// TransportInfo represents information about an object's transport status
type TransportInfo struct {
	PGMID          string             `json:"pgmid"`
	Object         string             `json:"object"`
	ObjectName     string             `json:"objectName"`
	Operation      string             `json:"operation"`
	DevClass       string             `json:"devClass"`
	Recording      string             `json:"recording"`
	Transports     []TransportRequest `json:"transports,omitempty"`
	LockedByUser   string             `json:"lockedByUser,omitempty"`
	LockedInTask   string             `json:"lockedInTask,omitempty"`
}

// --- Transport Operations ---

// GetUserTransports retrieves all transport requests for a user.
// Returns both workbench and customizing requests grouped by target system.
func (c *Client) GetUserTransports(ctx context.Context, userName string) (*UserTransports, error) {
	// Safety check
	if err := c.checkSafety(OpTransport, "GetUserTransports"); err != nil {
		return nil, err
	}

	userName = strings.ToUpper(userName)

	resp, err := c.transport.Request(ctx, "/sap/bc/adt/cts/transportrequests", &RequestOptions{
		Method: http.MethodGet,
		Query:  map[string][]string{"user": {userName}, "targets": {"true"}},
	})
	if err != nil {
		return nil, fmt.Errorf("get user transports failed: %w", err)
	}

	return parseUserTransports(resp.Body)
}

func parseUserTransports(data []byte) (*UserTransports, error) {
	// Strip namespace prefixes
	xmlStr := string(data)
	xmlStr = strings.ReplaceAll(xmlStr, "tm:", "")
	xmlStr = strings.ReplaceAll(xmlStr, "atom:", "")

	type transportObject struct {
		PGMID   string `xml:"pgmid,attr"`
		Type    string `xml:"type,attr"`
		Name    string `xml:"name,attr"`
		ObjInfo string `xml:"obj_info,attr"`
	}
	type task struct {
		Number  string            `xml:"number,attr"`
		Owner   string            `xml:"owner,attr"`
		Desc    string            `xml:"desc,attr"`
		Status  string            `xml:"status,attr"`
		Objects []transportObject `xml:"abap_object"`
	}
	type request struct {
		Number string `xml:"number,attr"`
		Owner  string `xml:"owner,attr"`
		Desc   string `xml:"desc,attr"`
		Status string `xml:"status,attr"`
		Tasks  []task `xml:"task"`
	}
	type target struct {
		Name      string    `xml:"name,attr"`
		Modifiable struct {
			Requests []request `xml:"request"`
		} `xml:"modifiable"`
		Released struct {
			Requests []request `xml:"request"`
		} `xml:"released"`
	}
	type root struct {
		Workbench struct {
			Targets []target `xml:"target"`
		} `xml:"workbench"`
		Customizing struct {
			Targets []target `xml:"target"`
		} `xml:"customizing"`
	}

	var resp root
	if err := xml.Unmarshal([]byte(xmlStr), &resp); err != nil {
		return nil, fmt.Errorf("parsing transport list: %w", err)
	}

	convertRequests := func(reqs []request, targetName string) []TransportRequest {
		var result []TransportRequest
		for _, r := range reqs {
			tr := TransportRequest{
				Number:      r.Number,
				Owner:       r.Owner,
				Description: r.Desc,
				Status:      r.Status,
				Target:      targetName,
			}
			for _, t := range r.Tasks {
				task := TransportTask{
					Number:      t.Number,
					Owner:       t.Owner,
					Description: t.Desc,
					Status:      t.Status,
				}
				for _, o := range t.Objects {
					task.Objects = append(task.Objects, TransportObject{
						PGMID:   o.PGMID,
						Type:    o.Type,
						Name:    o.Name,
						ObjInfo: o.ObjInfo,
					})
				}
				tr.Tasks = append(tr.Tasks, task)
			}
			result = append(result, tr)
		}
		return result
	}

	result := &UserTransports{}

	// Process workbench targets
	for _, t := range resp.Workbench.Targets {
		reqs := convertRequests(t.Modifiable.Requests, t.Name)
		for i := range reqs {
			reqs[i].Type = "workbench"
		}
		result.Workbench = append(result.Workbench, reqs...)

		releasedReqs := convertRequests(t.Released.Requests, t.Name)
		for i := range releasedReqs {
			releasedReqs[i].Type = "workbench"
		}
		result.Workbench = append(result.Workbench, releasedReqs...)
	}

	// Process customizing targets
	for _, t := range resp.Customizing.Targets {
		reqs := convertRequests(t.Modifiable.Requests, t.Name)
		for i := range reqs {
			reqs[i].Type = "customizing"
		}
		result.Customizing = append(result.Customizing, reqs...)
	}

	return result, nil
}

// GetTransportInfo retrieves transport information for an object.
// Returns available transports and whether the object is locked.
func (c *Client) GetTransportInfo(ctx context.Context, objectURL string, devClass string) (*TransportInfo, error) {
	// Safety check
	if err := c.checkSafety(OpTransport, "GetTransportInfo"); err != nil {
		return nil, err
	}

	body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
  <asx:values>
    <DATA>
      <DEVCLASS>%s</DEVCLASS>
      <OPERATION>I</OPERATION>
      <URI>%s</URI>
    </DATA>
  </asx:values>
</asx:abap>`, devClass, objectURL)

	resp, err := c.transport.Request(ctx, "/sap/bc/adt/cts/transportchecks", &RequestOptions{
		Method:      http.MethodPost,
		Body:        []byte(body),
		ContentType: "application/vnd.sap.as+xml; charset=UTF-8; dataname=com.sap.adt.transport.service.checkData",
		Accept:      "application/vnd.sap.as+xml;charset=UTF-8;dataname=com.sap.adt.transport.service.checkData",
	})
	if err != nil {
		return nil, fmt.Errorf("get transport info failed: %w", err)
	}

	return parseTransportInfo(resp.Body)
}

func parseTransportInfo(data []byte) (*TransportInfo, error) {
	// Basic parsing - extract main fields
	type dataType struct {
		PGMID      string `xml:"PGMID"`
		Object     string `xml:"OBJECT"`
		ObjectName string `xml:"OBJECTNAME"`
		Operation  string `xml:"OPERATION"`
		DevClass   string `xml:"DEVCLASS"`
		Recording  string `xml:"RECORDING"`
	}
	type values struct {
		Data dataType `xml:"DATA"`
	}
	type abap struct {
		Values values `xml:"values"`
	}

	// Strip namespace prefix
	xmlStr := strings.ReplaceAll(string(data), "asx:", "")

	var resp abap
	if err := xml.Unmarshal([]byte(xmlStr), &resp); err != nil {
		return nil, fmt.Errorf("parsing transport info: %w", err)
	}

	return &TransportInfo{
		PGMID:      resp.Values.Data.PGMID,
		Object:     resp.Values.Data.Object,
		ObjectName: resp.Values.Data.ObjectName,
		Operation:  resp.Values.Data.Operation,
		DevClass:   resp.Values.Data.DevClass,
		Recording:  resp.Values.Data.Recording,
	}, nil
}

// CreateTransport creates a new transport request.
// Returns the transport number on success.
func (c *Client) CreateTransport(ctx context.Context, objectURL string, description string, devClass string) (string, error) {
	// Safety check
	if err := c.checkSafety(OpTransport, "CreateTransport"); err != nil {
		return "", err
	}

	body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
  <asx:values>
    <DATA>
      <DEVCLASS>%s</DEVCLASS>
      <REQUEST_TEXT>%s</REQUEST_TEXT>
      <REF>%s</REF>
      <OPERATION>I</OPERATION>
    </DATA>
  </asx:values>
</asx:abap>`, devClass, description, objectURL)

	resp, err := c.transport.Request(ctx, "/sap/bc/adt/cts/transports", &RequestOptions{
		Method:      http.MethodPost,
		Body:        []byte(body),
		ContentType: "application/vnd.sap.as+xml; charset=UTF-8; dataname=com.sap.adt.CreateCorrectionRequest",
		Accept:      "text/plain",
	})
	if err != nil {
		return "", fmt.Errorf("create transport failed: %w", err)
	}

	// Response is a URL, extract transport number from the end
	transportURL := string(resp.Body)
	parts := strings.Split(transportURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1], nil
	}

	return "", fmt.Errorf("unexpected response format: %s", transportURL)
}

// ReleaseTransport releases a transport request.
// Returns release reports/messages.
func (c *Client) ReleaseTransport(ctx context.Context, transportNumber string, ignoreLocks bool) ([]string, error) {
	// Safety check
	if err := c.checkSafety(OpTransport, "ReleaseTransport"); err != nil {
		return nil, err
	}

	transportNumber = strings.ToUpper(transportNumber)

	// Validate transport number format (e.g., DEVK900001)
	if len(transportNumber) != 10 {
		return nil, fmt.Errorf("invalid transport number format: %s (expected 10 characters)", transportNumber)
	}

	action := "newreleasejobs"
	if ignoreLocks {
		action = "relwithignlock"
	}

	endpoint := fmt.Sprintf("/sap/bc/adt/cts/transportrequests/%s/%s", transportNumber, action)
	resp, err := c.transport.Request(ctx, endpoint, &RequestOptions{
		Method: http.MethodPost,
		Accept: "application/*",
	})
	if err != nil {
		return nil, fmt.Errorf("release transport failed: %w", err)
	}

	return parseReleaseResult(resp.Body)
}

func parseReleaseResult(data []byte) ([]string, error) {
	// Extract messages from release result
	xmlStr := string(data)
	xmlStr = strings.ReplaceAll(xmlStr, "tm:", "")
	xmlStr = strings.ReplaceAll(xmlStr, "chkrun:", "")

	type message struct {
		Type string `xml:"type,attr"`
		Text string `xml:"shortText,attr"`
	}
	type report struct {
		Reporter  string    `xml:"reporter,attr"`
		Status    string    `xml:"status,attr"`
		Messages  []message `xml:"checkMessageList>checkMessage"`
	}
	type root struct {
		Reports []report `xml:"releasereports>checkReport"`
	}

	var resp root
	if err := xml.Unmarshal([]byte(xmlStr), &resp); err != nil {
		// If parsing fails, return empty
		return []string{}, nil
	}

	var messages []string
	for _, r := range resp.Reports {
		messages = append(messages, fmt.Sprintf("[%s] Status: %s", r.Reporter, r.Status))
		for _, m := range r.Messages {
			messages = append(messages, fmt.Sprintf("  [%s] %s", m.Type, m.Text))
		}
	}

	return messages, nil
}
