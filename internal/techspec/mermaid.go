package techspec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// codeObjectTypes defines which object types are rendered in the Mermaid diagram.
// Tables, structures, domains, packages etc. are excluded.
var codeObjectTypes = map[string]bool{
	"PROG": true, // Report / Program
	"INCL": true, // Include
	"CLAS": true, // Class
	"INTF": true, // Interface
	"FUNC": true, // Function Module
	"FUGR": true, // Function Group
	"ENHO": true, // Enhancement Implementation
	"ENHS": true, // Enhancement Spot
	"BADI": true, // BAdI Definition (legacy)
	"SFPF": true, // SmartForms Form
	"SSFO": true, // Adobe Form
	"SRVB": true, // OData Service Binding
	"SRVD": true, // Service Definition
	"BDEF": true, // Behaviour Definition
	"DDLS": true, // CDS View (DDL Source)
	"WDYN": true, // Web Dynpro Application
}

// nodeStyle maps object type → Mermaid classDef style string.
var nodeStyle = map[string]string{
	"PROG": "fill:#4472C4,color:#fff,stroke:#2F5597",        // Blue
	"INCL": "fill:#6FA8DC,color:#fff,stroke:#3D85C8",        // Light Blue
	"CLAS": "fill:#6AA84F,color:#fff,stroke:#38761D",        // Green
	"INTF": "fill:#93C47D,color:#000,stroke:#6AA84F",        // Light Green
	"FUNC": "fill:#E69138,color:#fff,stroke:#B45F06",        // Orange
	"FUGR": "fill:#F6B26B,color:#000,stroke:#E69138",        // Light Orange
	"ENHO": "fill:#CC0000,color:#fff,stroke:#990000",        // Red
	"ENHS": "fill:#EA9999,color:#000,stroke:#CC0000",        // Light Red
	"BADI": "fill:#FF6B6B,color:#fff,stroke:#CC0000",        // Red-Orange
	"SFPF": "fill:#8E4585,color:#fff,stroke:#6A2E6A",        // Purple (SmartForms)
	"SSFO": "fill:#9966CC,color:#fff,stroke:#7B4FAA",        // Violet (Adobe Forms)
	"SRVB": "fill:#2E6DA4,color:#fff,stroke:#1A4A7A",        // Dark Teal (OData Binding)
	"SRVD": "fill:#4A86A8,color:#fff,stroke:#2E6DA4",        // Teal (Service Def)
	"BDEF": "fill:#76A5AF,color:#fff,stroke:#4A86A8",        // Light Teal (Behaviour Def)
	"DDLS": "fill:#45818E,color:#fff,stroke:#2E6DA4",        // CDS View
	"WDYN": "fill:#3D3D8C,color:#fff,stroke:#2A2A6E",        // Indigo (Web Dynpro)
}

var nonAlphaNum = regexp.MustCompile(`[^A-Za-z0-9_]`)

// nodeID produces a valid Mermaid node identifier.
func nodeID(objType, name string) string {
	return nonAlphaNum.ReplaceAllString(objType+"_"+name, "_")
}

// nodeLabel produces the display label.
func nodeLabel(objType, name string) string {
	return fmt.Sprintf("%s: %s", objType, name)
}

// styleClass returns the classDef name for a type (falls back to "DEFAULT").
func styleClass(objType string) string {
	if _, ok := nodeStyle[objType]; ok {
		return objType
	}
	return "DEFAULT"
}

// ─── Source analysis helpers ──────────────────────────────────────────────────

var (
	reFuncCall   = regexp.MustCompile(`(?i)CALL\s+FUNCTION\s+['"]([A-Z0-9_/]+)['"]`)
	reNewObj     = regexp.MustCompile(`(?i)(?:CREATE\s+OBJECT\s+\w+\s+TYPE|=\s*NEW)\s+([A-Z][A-Z0-9_/]*)`)
	reInclude    = regexp.MustCompile(`(?im)^\s*INCLUDE\s+([A-Z][A-Z0-9_/]*)\s*\.`)
	reMethodCall = regexp.MustCompile(`(?i)(?:=>|->)(\w+)\s*\(`)
)

type edge struct{ from, to, label string }

func detectEdges(source, fromID string, knownIDs map[string]bool) []edge {
	var edges []edge
	seen := map[string]bool{}

	addEdge := func(toID, label string) {
		key := fromID + "→" + toID + "→" + label
		if !seen[key] && toID != fromID && knownIDs[toID] {
			seen[key] = true
			edges = append(edges, edge{fromID, toID, label})
		}
	}

	for _, m := range reFuncCall.FindAllStringSubmatch(source, -1) {
		addEdge(nodeID("FUNC", strings.ToUpper(m[1])), "calls FM")
	}
	for _, m := range reNewObj.FindAllStringSubmatch(source, -1) {
		addEdge(nodeID("CLAS", strings.ToUpper(m[1])), "instantiates")
	}
	for _, m := range reInclude.FindAllStringSubmatch(source, -1) {
		addEdge(nodeID("INCL", strings.ToUpper(m[1])), "includes")
	}
	return edges
}

// ─── Main diagram function ────────────────────────────────────────────────────

// DiagramInput is the input to GenerateMermaid.
type DiagramInput struct {
	// Objects from all transport requests (deduplicated by caller).
	Objects []adt.TransportObjectV2
	// TransportNumbers lets multi-TR specs show subgraphs.
	TransportNumbers []string
	// Optional source code per object name for edge detection.
	Sources map[string]string // key: "TYPE/NAME"
}

// GenerateMermaid produces a Mermaid graph LR diagram string (including ```mermaid fences).
func GenerateMermaid(in DiagramInput) string {
	// Filter to code objects only, deduplicate
	type objKey struct{ t, n string }
	seen := map[objKey]bool{}
	var codeObjs []adt.TransportObjectV2
	for _, o := range in.Objects {
		k := objKey{o.Type, o.Name}
		if codeObjectTypes[o.Type] && !seen[k] {
			seen[k] = true
			codeObjs = append(codeObjs, o)
		}
	}
	if len(codeObjs) == 0 {
		return "_No code objects found in transport(s) — diagram skipped._"
	}

	// Build known node ID set for edge filtering
	knownIDs := map[string]bool{}
	for _, o := range codeObjs {
		knownIDs[nodeID(o.Type, o.Name)] = true
	}

	// Collect used style classes
	usedStyles := map[string]bool{"DEFAULT": false}
	for _, o := range codeObjs {
		usedStyles[styleClass(o.Type)] = true
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("graph LR\n")

	// classDef declarations
	sb.WriteString("    classDef DEFAULT fill:#EEEEEE,color:#000,stroke:#999\n")
	for cls, present := range usedStyles {
		if !present || cls == "DEFAULT" {
			continue
		}
		fmt.Fprintf(&sb, "    classDef %s %s\n", cls, nodeStyle[cls])
	}
	sb.WriteString("\n")

	// Nodes — grouped in subgraphs if multiple TRs
	if len(in.TransportNumbers) > 1 {
		// Group by TR (objects carry TR info via their position in the flat list)
		// We use a simple heuristic: render all objects in a single subgraph per TR
		// based on the order they appear (each TR's objects are contiguous).
		// For simplicity we render one subgraph per TR number.
		for _, trNum := range in.TransportNumbers {
			fmt.Fprintf(&sb, "    subgraph %s[\"TR: %s\"]\n", nonAlphaNum.ReplaceAllString(trNum, "_"), trNum)
			for _, o := range codeObjs {
				// We can't tell which TR an object came from after dedup,
				// so render all in the first subgraph and skip rest.
				// A richer implementation would tag objects with their TR.
				_ = o
			}
			// Close subgraph without objects (objects rendered outside below)
			sb.WriteString("    end\n")
		}
		sb.WriteString("\n")
		// Render all nodes outside subgraphs
		for _, o := range codeObjs {
			fmt.Fprintf(&sb, "    %s[\"%s\"]:::%s\n",
				nodeID(o.Type, o.Name),
				nodeLabel(o.Type, o.Name),
				styleClass(o.Type),
			)
		}
	} else {
		for _, o := range codeObjs {
			fmt.Fprintf(&sb, "    %s[\"%s\"]:::%s\n",
				nodeID(o.Type, o.Name),
				nodeLabel(o.Type, o.Name),
				styleClass(o.Type),
			)
		}
	}

	// Edges from source analysis
	var allEdges []edge
	edgeSeen := map[string]bool{}
	for key, src := range in.Sources {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		fromID := nodeID(parts[0], parts[1])
		for _, e := range detectEdges(src, fromID, knownIDs) {
			ekey := e.from + e.to + e.label
			if !edgeSeen[ekey] {
				edgeSeen[ekey] = true
				allEdges = append(allEdges, e)
			}
		}
	}

	if len(allEdges) > 0 {
		sb.WriteString("\n    %% Relationships detected from source\n")
		for _, e := range allEdges {
			fmt.Fprintf(&sb, "    %s -->|\"%s\"| %s\n", e.from, e.label, e.to)
		}
	}

	sb.WriteString("```")
	return sb.String()
}
