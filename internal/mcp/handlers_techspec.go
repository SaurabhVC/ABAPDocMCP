// Package mcp provides the MCP server implementation for ABAP ADT tools.
// handlers_techspec.go contains the handler for WRICEF Technical Specification generation.
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oisee/vibing-steampunk/internal/techspec"
	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// handleGenerateWricefTechSpec builds a WRICEF Technical Specification Markdown document
// from one or more transport request numbers.
func (s *Server) handleGenerateWricefTechSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// ── Required: transport numbers ──────────────────────────────────────────
	transportNumbersRaw, ok := request.Params.Arguments["transport_numbers"].(string)
	if !ok || strings.TrimSpace(transportNumbersRaw) == "" {
		return newToolResultError("transport_numbers is required (comma-separated TR numbers, e.g. 'DEVK900123,DEVK900124')"), nil
	}

	var trNumbers []string
	for _, tr := range strings.Split(transportNumbersRaw, ",") {
		tr = strings.TrimSpace(tr)
		if tr != "" {
			trNumbers = append(trNumbers, strings.ToUpper(tr))
		}
	}
	if len(trNumbers) == 0 {
		return newToolResultError("at least one transport number is required"), nil
	}

	// ── WRICEF type ───────────────────────────────────────────────────────────
	wricefTypeRaw, _ := request.Params.Arguments["wricef_type"].(string)
	wricefType := techspec.WricefType(strings.ToUpper(strings.TrimSpace(wricefTypeRaw)))
	switch wricefType {
	case techspec.WricefReport, techspec.WricefInterface, techspec.WricefConversion,
		techspec.WricefEnhancement, techspec.WricefForm, techspec.WricefWorkflow:
		// valid
	default:
		wricefType = techspec.WricefReport // sensible default
	}

	// ── Optional metadata params ──────────────────────────────────────────────
	strParam := func(key string) string {
		v, _ := request.Params.Arguments[key].(string)
		return strings.TrimSpace(v)
	}

	params := techspec.Params{
		WricefType: wricefType,
		WricefID:   strParam("wricef_id"),
		Complexity: strParam("complexity"),
		CRNumber:   strParam("cr_number"),
		Module:     strParam("module"),

		DeveloperName:   strParam("developer_name"),
		DeveloperUserID: strParam("developer_user_id"),
		ReviewerName:    strParam("reviewer_name"),
		ApproverName:    strParam("approver_name"),

		SystemID: strParam("system_id"),
		Client:   strParam("client"),
		Version:  strParam("version"),

		// Interface-specific
		InterfaceDirection: strParam("interface_direction"),
		InterfaceProtocol:  strParam("interface_protocol"),
		InterfaceFrequency: strParam("interface_frequency"),

		// Enhancement-specific
		EnhancementType:    strParam("enhancement_type"),
		EnhancementName:    strParam("enhancement_name"),
		OriginalProgram:    strParam("original_program"),

		// Form-specific
		FormTechnology: strParam("form_technology"),
		FormOutputType: strParam("form_output_type"),

		// Workflow-specific
		WorkflowSteps: strParam("workflow_steps"),

		// OData/RAP-specific
		ServiceBinding:    strParam("service_binding"),
		ServiceDefinition: strParam("service_definition"),

		Notes: strParam("notes"),
	}

	// ── fetch_source_code flag ────────────────────────────────────────────────
	fetchSource := true // default on
	if v, ok := request.Params.Arguments["fetch_source_code"].(bool); ok {
		fetchSource = v
	}

	// ── Fetch transport details ───────────────────────────────────────────────
	var transports []*adt.TransportDetails
	var fetchErrors []string

	for _, trNum := range trNumbers {
		td, err := s.adtClient.GetTransport(ctx, trNum)
		if err != nil {
			fetchErrors = append(fetchErrors, fmt.Sprintf("GetTransport(%s): %v", trNum, err))
			continue
		}
		transports = append(transports, td)
	}

	if len(transports) == 0 {
		msg := "Failed to fetch any transport details"
		if len(fetchErrors) > 0 {
			msg += ":\n" + strings.Join(fetchErrors, "\n")
		}
		return newToolResultError(msg), nil
	}

	// ── Optionally fetch source code ──────────────────────────────────────────
	var sources []techspec.ObjectSource

	if fetchSource {
		// Collect unique code objects across all transports
		type objKey struct{ typ, name string }
		seen := make(map[objKey]bool)

		for _, td := range transports {
			for _, obj := range td.Objects {
				key := objKey{typ: strings.ToUpper(obj.Type), name: strings.ToUpper(obj.Name)}
				if seen[key] {
					continue
				}
				seen[key] = true

				// Only fetch source for code object types we care about
				switch key.typ {
				case "PROG", "INCL", "CLAS", "INTF", "FUNC", "FUGR", "DDLS", "BDEF", "SRVD", "SRVB":
					src, err := s.adtClient.GetSource(ctx, key.typ, key.name, nil)
					if err != nil {
						// non-fatal: record and continue
						fetchErrors = append(fetchErrors, fmt.Sprintf("GetSource(%s %s): %v", key.typ, key.name, err))
						continue
					}
					sources = append(sources, techspec.ObjectSource{
						ObjType: key.typ,
						Name:    key.name,
						Source:  src,
					})
				}
			}
		}
	}

	// ── Build generator input and produce Markdown ────────────────────────────
	in := techspec.Input{
		Params:     params,
		Transports: transports,
		Sources:    sources,
	}

	markdown := techspec.Generate(in)

	// ── Append non-fatal warnings at the end ─────────────────────────────────
	if len(fetchErrors) > 0 {
		markdown += "\n\n---\n\n> **Warnings during generation:**\n"
		for _, e := range fetchErrors {
			markdown += "> - " + e + "\n"
		}
	}

	return mcp.NewToolResultText(markdown), nil
}
