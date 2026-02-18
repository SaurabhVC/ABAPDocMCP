package techspec

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

// ─── Input types ─────────────────────────────────────────────────────────────

// WricefType represents the WRICEF classification.
type WricefType string

const (
	WricefReport      WricefType = "REPORT"
	WricefInterface   WricefType = "INTERFACE"
	WricefConversion  WricefType = "CONVERSION"
	WricefEnhancement WricefType = "ENHANCEMENT"
	WricefForm        WricefType = "FORM"
	WricefWorkflow    WricefType = "WORKFLOW"
)

var wricefLabel = map[WricefType]string{
	WricefReport:      "R — Report",
	WricefInterface:   "I — Interface",
	WricefConversion:  "C — Conversion",
	WricefEnhancement: "E — Enhancement",
	WricefForm:        "F — Form",
	WricefWorkflow:    "W — Workflow",
}

// Params holds all user-supplied metadata for the tech spec.
type Params struct {
	WricefType  WricefType
	WricefID    string // e.g. "ZSD-R-001"
	Complexity  string // S / M / L / XL
	CRNumber    string // change request / ticket
	Module      string // functional module (SD, FI, MM ...)

	DeveloperName   string
	DeveloperUserID string
	ReviewerName    string
	ApproverName    string

	SystemID string
	Client   string
	Version  string

	// Interface-specific
	InterfaceDirection string // Inbound / Outbound / Bidirectional
	InterfaceProtocol  string // RFC / IDoc / REST / SOAP / File
	InterfaceFrequency string // Real-time / Batch / Event-driven

	// Enhancement-specific
	EnhancementType    string // BADI / User Exit / Implicit Enhancement
	EnhancementName    string // BADI / exit name
	OriginalProgram    string // program being enhanced

	// Form-specific
	FormTechnology string // SmartForms / Adobe Forms / SAPscript
	FormOutputType string // Spool / Email / PDF

	// Workflow-specific
	WorkflowSteps string // newline-separated steps

	// OData / RAP-specific
	ServiceBinding    string
	ServiceDefinition string

	Notes string
}

// ObjectSource holds fetched source code for one object.
type ObjectSource struct {
	ObjType string
	Name    string
	Source  string
}

// ATCSummary is a lightweight ATC result per object.
type ATCSummary struct {
	ObjectName string
	Errors     int
	Warnings   int
	Info       int
	Findings   []string // short description lines
}

// UnitTestSummary holds unit test results per object.
type UnitTestSummary struct {
	ObjectName string
	Passed     int
	Failed     int
	Skipped    int
}

// Input is the full input to Generate.
type Input struct {
	Params     Params
	Transports []*adt.TransportDetails // one per TR number

	// Optional enrichments (may be partially filled depending on fetchSourceCode)
	Sources      []ObjectSource
	ATCResults   []ATCSummary
	UnitTests    []UnitTestSummary
	CallGraphMD  string // pre-formatted Markdown from GetCallGraph
	ObjectTreeMD string // pre-formatted Markdown from GetObjectStructure
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func today() string { return time.Now().Format("2006-01-02") }

func h2(title string) string { return "\n## " + title + "\n" }
func h3(title string) string { return "\n### " + title + "\n" }

func mdTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return "_None detected._\n"
	}
	head := "| " + strings.Join(headers, " | ") + " |"
	sep := "| " + strings.Join(repeatStr("---", len(headers)), " | ") + " |"
	var lines []string
	lines = append(lines, head, sep)
	for _, row := range rows {
		lines = append(lines, "| "+strings.Join(row, " | ")+" |")
	}
	return strings.Join(lines, "\n") + "\n"
}

func repeatStr(s string, n int) []string {
	r := make([]string, n)
	for i := range r {
		r[i] = s
	}
	return r
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// collectAllObjects returns de-duplicated objects across all TRs.
func collectAllObjects(transports []*adt.TransportDetails) []adt.TransportObjectV2 {
	type key struct{ t, n string }
	seen := map[key]bool{}
	var out []adt.TransportObjectV2
	for _, tr := range transports {
		for _, o := range tr.Objects {
			k := key{o.Type, o.Name}
			if !seen[k] {
				seen[k] = true
				out = append(out, o)
			}
		}
	}
	return out
}

// ─── Source analysis helpers ──────────────────────────────────────────────────

var (
	reDB   = regexp.MustCompile(`(?i)(?:SELECT|INSERT\s+INTO|UPDATE|DELETE\s+FROM|MODIFY)\s+(?:SINGLE\s+)?(?:\*\s+)?(?:[\w,\s]+\s+)?(?:FROM|INTO)\s+([A-Z]\w+)`)
	reFM   = regexp.MustCompile(`(?i)CALL\s+FUNCTION\s+['"]([A-Z0-9_/]+)['"]`)
	reMeth = regexp.MustCompile(`(?i)(?:=>|->)(\w+)\s*\(`)
	reAuth = regexp.MustCompile(`(?i)AUTHORITY-CHECK\s+OBJECT\s+['"]([A-Z0-9_/]+)['"]`)
	reSYRC = regexp.MustCompile(`(?i)IF\s+SY-SUBRC\s*<>\s*0`)
	reMsg  = regexp.MustCompile(`(?i)MESSAGE\s+\S+\s+TYPE\s+['"]([EWI])['"]`)
	rePerf = regexp.MustCompile(`(?i)SELECT\s+\*`)
	rePerfLoop = regexp.MustCompile(`(?im)LOOP\s+AT[\s\S]{1,80}SELECT`)
	reParams   = regexp.MustCompile(`(?im)^\s*PARAMETERS\s+(\w+)\s+(.+?)(?:\.|$)`)
	reSelOpts  = regexp.MustCompile(`(?im)^\s*SELECT-OPTIONS\s+(\w+)\s+FOR\s+(\S+)`)
)

func extractFromSources(sources []ObjectSource) (dbs, fms, methods, authObjs, errors, perfIssues []string, selParams [][]string) {
	dbSeen := map[string]bool{}
	fmSeen := map[string]bool{}
	mSeen := map[string]bool{}
	aSeen := map[string]bool{}
	eSeen := map[string]bool{}

	for _, s := range sources {
		src := s.Source

		for _, m := range reDB.FindAllStringSubmatch(src, -1) {
			t := strings.ToUpper(m[1])
			if !dbSeen[t] { dbSeen[t] = true; dbs = append(dbs, t) }
		}
		for _, m := range reFM.FindAllStringSubmatch(src, -1) {
			f := strings.ToUpper(m[1])
			if !fmSeen[f] { fmSeen[f] = true; fms = append(fms, f) }
		}
		for _, m := range reMeth.FindAllStringSubmatch(src, -1) {
			meth := strings.ToUpper(m[1])
			if !mSeen[meth] { mSeen[meth] = true; methods = append(methods, meth) }
		}
		for _, m := range reAuth.FindAllStringSubmatch(src, -1) {
			a := strings.ToUpper(m[1])
			if !aSeen[a] { aSeen[a] = true; authObjs = append(authObjs, a) }
		}
		if reSYRC.MatchString(src) {
			e := "SY-SUBRC <> 0 check(s) present"
			if !eSeen[e] { eSeen[e] = true; errors = append(errors, e) }
		}
		for _, m := range reMsg.FindAllStringSubmatch(src, -1) {
			mtype := m[1]
			label := map[string]string{"E": "Error messages (TYPE 'E')", "W": "Warning messages (TYPE 'W')", "I": "Info messages (TYPE 'I')"}[strings.ToUpper(mtype)]
			if label != "" && !eSeen[label] { eSeen[label] = true; errors = append(errors, label) }
		}
		if rePerf.MatchString(src) {
			perfIssues = append(perfIssues, "`SELECT *` detected — select only required fields.")
		}
		if rePerfLoop.MatchString(src) {
			perfIssues = append(perfIssues, "`SELECT` inside `LOOP` detected — use FOR ALL ENTRIES or JOIN.")
		}
		for _, m := range reParams.FindAllStringSubmatch(src, -1) {
			selParams = append(selParams, []string{"`" + m[1] + "`", "PARAMETERS", m[2]})
		}
		for _, m := range reSelOpts.FindAllStringSubmatch(src, -1) {
			selParams = append(selParams, []string{"`" + m[1] + "`", "SELECT-OPTIONS", "For `" + m[2] + "`"})
		}
	}
	return
}

// ─── Section renderers ────────────────────────────────────────────────────────

func renderDocumentInfo(in *Input) string {
	trNums := make([]string, len(in.Transports))
	for i, tr := range in.Transports { trNums[i] = tr.Number }
	label := wricefLabel[in.Params.WricefType]
	if label == "" { label = string(in.Params.WricefType) }
	return mdTable([]string{"Field", "Value"}, [][]string{
		{"Document Title", orDash(in.Params.WricefID) + " — Technical Specification"},
		{"WRICEF ID", orDash(in.Params.WricefID)},
		{"WRICEF Type", label},
		{"Complexity", orDash(in.Params.Complexity)},
		{"SAP System", orDash(in.Params.SystemID)},
		{"Client", orDash(in.Params.Client)},
		{"Module", orDash(in.Params.Module)},
		{"Document Version", orDash(in.Params.Version)},
		{"Created Date", today()},
		{"Transport Number(s)", func() string {
			if len(trNums) == 0 { return "—" }
			return strings.Join(trNums, ", ")
		}()},
	})
}

func renderDeveloperInfo(in *Input) string {
	return mdTable([]string{"Role", "Name", "User ID"}, [][]string{
		{"Developer", orDash(in.Params.DeveloperName), orDash(in.Params.DeveloperUserID)},
		{"Reviewer", orDash(in.Params.ReviewerName), "—"},
		{"Approver", orDash(in.Params.ApproverName), "—"},
	})
}

func renderChangeRequest(in *Input) string {
	return mdTable([]string{"Field", "Value"}, [][]string{
		{"CR / Ticket Number", orDash(in.Params.CRNumber)},
		{"Business Justification", "_(to be filled)_"},
		{"Linked Documents", "_(functional spec, BRD link)_"},
	})
}

func renderWricefClassification(in *Input) string {
	label := wricefLabel[in.Params.WricefType]
	if label == "" { label = string(in.Params.WricefType) }
	return mdTable([]string{"Field", "Value"}, [][]string{
		{"WRICEF Category", label},
		{"Complexity", orDash(in.Params.Complexity)},
		{"Estimated Effort", "_(to be filled)_"},
		{"Module / Functional Area", orDash(in.Params.Module)},
	})
}

func renderTransportDetails(in *Input) string {
	if len(in.Transports) == 0 {
		return "_No transport information available._\n"
	}
	var parts []string
	for _, tr := range in.Transports {
		parts = append(parts, mdTable([]string{"Property", "Value"}, [][]string{
			{"Transport Number", tr.Number},
			{"Description", orDash(tr.Description)},
			{"Owner", orDash(tr.Owner)},
			{"Status", orDash(tr.StatusText)},
			{"Target", orDash(tr.Target)},
			{"Changed At", orDash(tr.ChangedAt)},
		}))
	}
	return strings.Join(parts, "\n\n")
}

func renderTransportContents(in *Input) string {
	allObjs := collectAllObjects(in.Transports)
	if len(allObjs) == 0 {
		return "_No objects found in transport(s)._\n"
	}
	rows := make([][]string, len(allObjs))
	for i, o := range allObjs {
		rows[i] = []string{
			fmt.Sprintf("%d", i+1),
			"`" + o.Name + "`",
			o.Type,
			orDash(o.PgmID),
			orDash(o.Info),
		}
	}
	return mdTable([]string{"#", "Object Name", "Type", "PGMID", "Description"}, rows)
}

func renderBusinessRequirement() string {
	return "_Describe the business need, functional requirement, or process gap this development addresses._\n\n" +
		"**Functional Specification Reference:** _(link or document number)_\n"
}

func renderScopeAssumptions() string {
	return "**In Scope:**\n- _(list scope items)_\n\n" +
		"**Out of Scope:**\n- _(list exclusions)_\n\n" +
		"**Assumptions:**\n- _(list assumptions)_\n\n" +
		"**Dependencies:**\n- _(other objects, third-party systems, config)_\n"
}

func renderTechnicalDesign(in *Input) string {
	allObjs := collectAllObjects(in.Transports)
	typeSet := map[string]bool{}
	for _, o := range allObjs { typeSet[o.Type] = true }
	types := make([]string, 0, len(typeSet))
	for t := range typeSet { types = append(types, t) }

	names := make([]string, 0, len(in.Sources))
	for _, s := range in.Sources { names = append(names, fmt.Sprintf("`%s` (%s)", s.Name, s.ObjType)) }

	var sb strings.Builder
	if len(names) > 0 {
		fmt.Fprintf(&sb, "**Objects:** %s\n\n", strings.Join(names, ", "))
	}
	fmt.Fprintf(&sb, "**Object Types:** %s\n\n", strings.Join(types, ", "))
	sb.WriteString("**Approach:** _(describe technical approach, major design decisions, and SAP standard objects / BAPIs reused)_\n")
	return sb.String()
}

func renderObjectStructure(in *Input) string {
	if in.ObjectTreeMD != "" {
		return in.ObjectTreeMD + "\n"
	}
	return "_Object structure not fetched. Set `fetch_source` to true to enable this section._\n"
}

func renderTypeHierarchy(in *Input) string {
	return "_Type hierarchy details — available for OO objects (CLAS/INTF). Requires `GetTypeHierarchy` call._\n\n" +
		"| Level | Type | Name | Kind |\n| --- | --- | --- | --- |\n| _(supertypes above, subtypes below)_ | | | |\n"
}

func renderObjectRelationDiagram(in *Input) string {
	allObjs := collectAllObjects(in.Transports)
	trNums := make([]string, len(in.Transports))
	for i, tr := range in.Transports { trNums[i] = tr.Number }

	srcMap := map[string]string{}
	for _, s := range in.Sources {
		srcMap[s.ObjType+"/"+s.Name] = s.Source
	}

	return GenerateMermaid(DiagramInput{
		Objects:          allObjs,
		TransportNumbers: trNums,
		Sources:          srcMap,
	})
}

// WRICEF-specific sections ─────────────────────────────────────────────────────

func renderReportDetails(in *Input, selParams [][]string) string {
	var sb strings.Builder
	if len(selParams) > 0 {
		sb.WriteString(h3("Selection Screen / Input Parameters"))
		sb.WriteString(mdTable([]string{"Parameter", "Kind", "Definition"}, selParams))
	} else {
		sb.WriteString(h3("Selection Screen / Input Parameters"))
		sb.WriteString("_No PARAMETERS or SELECT-OPTIONS detected._\n")
	}
	sb.WriteString(h3("Output / ALV Layout"))
	sb.WriteString("**Output Type:** _(ALV Grid / ALV List / SmartForm / PDF / Excel download)_\n\n")
	sb.WriteString("| Column / Field | Data Element | Description |\n| --- | --- | --- |\n| _(field)_ | _(DTEL)_ | _(description)_ |\n")
	return sb.String()
}

func renderInterfaceDetails(in *Input) string {
	return mdTable([]string{"Property", "Value"}, [][]string{
		{"Direction", orDash(in.Params.InterfaceDirection)},
		{"Protocol", orDash(in.Params.InterfaceProtocol)},
		{"Frequency", orDash(in.Params.InterfaceFrequency)},
		{"Source System", "_(to be filled)_"},
		{"Target System", "_(to be filled)_"},
		{"Error Strategy", "_(to be filled)_"},
	})
}

func renderMessageMapping() string {
	return "| Source Field | Source Type | Target Field | Target Type | Transformation |\n" +
		"| --- | --- | --- | --- | --- |\n" +
		"| _(field)_ | _(type)_ | _(field)_ | _(type)_ | _(rule)_ |\n"
}

func renderEnhancementDetails(in *Input) string {
	return mdTable([]string{"Property", "Value"}, [][]string{
		{"Enhancement Type", orDash(in.Params.EnhancementType)},
		{"Enhancement Name", orDash(in.Params.EnhancementName)},
		{"Original Program", orDash(in.Params.OriginalProgram)},
		{"Impact on Standard", "_(describe)_"},
		{"Activation Required", "Yes / No"},
	})
}

func renderFormDetails(in *Input) string {
	return mdTable([]string{"Property", "Value"}, [][]string{
		{"Form Technology", orDash(in.Params.FormTechnology)},
		{"Output Type", orDash(in.Params.FormOutputType)},
		{"Print Program", "_(to be filled)_"},
		{"Output Type (NAST)", "_(to be filled)_"},
		{"Number of Pages", "—"},
	})
}

func renderWorkflowDetails(in *Input) string {
	var sb strings.Builder
	if in.Params.WorkflowSteps != "" {
		sb.WriteString("**Workflow Steps:**\n")
		for _, step := range strings.Split(in.Params.WorkflowSteps, "\n") {
			fmt.Fprintf(&sb, "- %s\n", strings.TrimSpace(step))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("| Step | Agent / Role | Task / Method | Event | Escalation |\n")
	sb.WriteString("| --- | --- | --- | --- | --- |\n")
	sb.WriteString("| _(step)_ | _(role/user)_ | _(task)_ | _(event)_ | _(deadline)_ |\n")
	return sb.String()
}

func renderODataDetails(in *Input) string {
	return mdTable([]string{"Property", "Value"}, [][]string{
		{"Service Binding", orDash(in.Params.ServiceBinding)},
		{"Service Definition", orDash(in.Params.ServiceDefinition)},
		{"Entity Sets", "_(to be filled)_"},
		{"Exposed Operations", "_(Create / Read / Update / Delete / Action)_"},
		{"Binding Version", "0001"},
	})
}

// Processing Logic sections ───────────────────────────────────────────────────

func renderPseudocode(in *Input) string {
	if len(in.Sources) == 0 {
		return "_Source code not fetched. Enable `fetch_source` to generate pseudocode._\n"
	}
	var parts []string
	for _, s := range in.Sources {
		code := generateSimplePseudo(s.Source)
		parts = append(parts, fmt.Sprintf("**%s / %s**\n\n```\n%s\n```", s.ObjType, s.Name, code))
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func generateSimplePseudo(source string) string {
	lines := strings.Split(source, "\n")
	var out []string
	indent := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "\"") {
			continue
		}
		upper := strings.ToUpper(line)

		// Dedent
		for _, kw := range []string{"ENDLOOP", "ENDIF", "ENDFORM", "ENDCASE", "ENDTRY", "ENDMETHOD", "ENDCLASS"} {
			if strings.HasPrefix(upper, kw) { indent-- }
		}
		if indent < 0 { indent = 0 }

		pad := strings.Repeat("  ", indent)
		out = append(out, pad+line)

		// Indent
		for _, kw := range []string{"LOOP AT", "IF ", "CASE ", "FORM ", "TRY.", "METHOD ", "CLASS "} {
			if strings.HasPrefix(upper, kw) { indent++ }
		}
	}
	if len(out) == 0 { return "_(source too short to parse)_" }
	if len(out) > 60 { out = append(out[:60], "... (truncated)") }
	return strings.Join(out, "\n")
}

func renderCallGraph(in *Input) string {
	if in.CallGraphMD != "" {
		return in.CallGraphMD + "\n"
	}
	return "_Call graph not fetched. Enable `fetch_call_graph` to populate this section._\n"
}

func renderDatabaseObjects(dbs []string) string {
	if len(dbs) == 0 { return "_No database objects detected._\n" }
	rows := make([][]string, len(dbs))
	for i, t := range dbs { rows[i] = []string{"`" + t + "`", "SELECT / MODIFY", "—"} }
	return mdTable([]string{"Table / View", "Access Type", "Description"}, rows)
}

func renderCalledFunctions(fms, methods []string) string {
	var rows [][]string
	for _, f := range fms    { rows = append(rows, []string{"`" + f + "`", "Function Module", "—"}) }
	for _, m := range methods { rows = append(rows, []string{"`" + m + "`", "Method", "—"}) }
	if len(rows) == 0 { return "_No function module / method calls detected._\n" }
	return mdTable([]string{"Name", "Type", "Purpose"}, rows)
}

func renderAuthorizationObjects(authObjs []string) string {
	if len(authObjs) == 0 { return "_No AUTHORITY-CHECK statements detected._\n" }
	rows := make([][]string, len(authObjs))
	for i, a := range authObjs { rows[i] = []string{"`" + a + "`", "—", "—"} }
	return mdTable([]string{"Authorization Object", "Fields / Activity", "Description"}, rows)
}

func renderATCFindings(in *Input) string {
	if len(in.ATCResults) == 0 {
		return "_ATC check not run. Enable `run_atc` to populate this section._\n"
	}
	var rows [][]string
	for _, r := range in.ATCResults {
		rows = append(rows, []string{r.ObjectName, fmt.Sprintf("%d", r.Errors), fmt.Sprintf("%d", r.Warnings), fmt.Sprintf("%d", r.Info)})
		for _, f := range r.Findings {
			rows = append(rows, []string{"  ↳", "", "", f})
		}
	}
	return mdTable([]string{"Object", "Errors", "Warnings", "Info / Findings"}, rows)
}

func renderUnitTestResults(in *Input) string {
	if len(in.UnitTests) == 0 {
		return "_Unit tests not run. Enable `run_unit_tests` to populate this section._\n"
	}
	rows := make([][]string, len(in.UnitTests))
	for i, r := range in.UnitTests {
		total := r.Passed + r.Failed + r.Skipped
		status := "✅ Pass"
		if r.Failed > 0 { status = "❌ Fail" }
		rows[i] = []string{r.ObjectName, fmt.Sprintf("%d", total), fmt.Sprintf("%d", r.Passed), fmt.Sprintf("%d", r.Failed), status}
	}
	return mdTable([]string{"Object", "Total", "Passed", "Failed", "Status"}, rows)
}

func renderErrorHandling(errors []string) string {
	if len(errors) == 0 { return "_No error handling patterns detected._\n" }
	rows := make([][]string, len(errors))
	for i, e := range errors { rows[i] = []string{e, "—"} }
	return mdTable([]string{"Scenario / Pattern", "Handling Strategy"}, rows)
}

func renderPerformanceConsiderations(issues []string) string {
	if len(issues) == 0 { return "_No automated performance concerns detected. Review manually._\n" }
	var sb strings.Builder
	for _, iss := range issues { fmt.Fprintf(&sb, "- %s\n", iss) }
	return sb.String()
}

func renderTestScenarios() string {
	return mdTable(
		[]string{"#", "Test Case", "Input / Condition", "Expected Result", "Status"},
		[][]string{
			{"1", "Positive scenario",         "—", "—", "⬜ Pending"},
			{"2", "Negative / error scenario",  "—", "—", "⬜ Pending"},
			{"3", "Boundary / edge case",       "—", "—", "⬜ Pending"},
			{"4", "Authorization check",        "—", "—", "⬜ Pending"},
			{"5", "Performance / volume test",  "—", "—", "⬜ Pending"},
		},
	)
}

func renderChangeHistory(in *Input) string {
	return mdTable(
		[]string{"Version", "Date", "Changed By", "User ID", "Description"},
		[][]string{{orDash(in.Params.Version), today(), orDash(in.Params.DeveloperName), orDash(in.Params.DeveloperUserID), "Initial version"}},
	)
}

func renderSignOff(in *Input) string {
	return mdTable([]string{"Role", "Name", "Date", "Signature"}, [][]string{
		{"Developer", orDash(in.Params.DeveloperName), "—", "—"},
		{"Reviewer",  orDash(in.Params.ReviewerName),  "—", "—"},
		{"Approver",  orDash(in.Params.ApproverName),  "—", "—"},
	})
}

// ─── Main Generator ───────────────────────────────────────────────────────────

// Generate produces the full Markdown technical specification.
func Generate(in Input) string {
	sections := ResolvedSections()

	label := wricefLabel[in.Params.WricefType]
	if label == "" { label = string(in.Params.WricefType) }

	dbs, fms, methods, authObjs, errors, perfIssues, selParams := extractFromSources(in.Sources)

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Technical Specification\n## %s — %s\n\n---\n", label, orDash(in.Params.WricefID))

	for _, sec := range sections {
		title := ResolvedTitle(sec)

		switch sec {
		case SectionDocumentInfo:
			sb.WriteString(h2(title)); sb.WriteString(renderDocumentInfo(&in))
		case SectionDeveloperInfo:
			sb.WriteString(h2(title)); sb.WriteString(renderDeveloperInfo(&in))
		case SectionChangeRequest:
			sb.WriteString(h2(title)); sb.WriteString(renderChangeRequest(&in))
		case SectionWricefClassification:
			sb.WriteString(h2(title)); sb.WriteString(renderWricefClassification(&in))
		case SectionTransportDetails:
			sb.WriteString(h2(title)); sb.WriteString(renderTransportDetails(&in))
		case SectionTransportContents:
			sb.WriteString(h2(title)); sb.WriteString(renderTransportContents(&in))
		case SectionBusinessRequirement:
			sb.WriteString(h2(title)); sb.WriteString(renderBusinessRequirement())
		case SectionScopeAssumptions:
			sb.WriteString(h2(title)); sb.WriteString(renderScopeAssumptions())
		case SectionTechnicalDesign:
			sb.WriteString(h2(title)); sb.WriteString(renderTechnicalDesign(&in))
		case SectionObjectStructure:
			sb.WriteString(h2(title)); sb.WriteString(renderObjectStructure(&in))
		case SectionTypeHierarchy:
			sb.WriteString(h2(title)); sb.WriteString(renderTypeHierarchy(&in))
		case SectionObjectRelationDiagram:
			sb.WriteString(h2(title))
			sb.WriteString(renderObjectRelationDiagram(&in))
			sb.WriteString("\n")
		case SectionReportDetails:
			sb.WriteString(h2(title)); sb.WriteString(renderReportDetails(&in, selParams))
		case SectionInterfaceDetails:
			sb.WriteString(h2(title)); sb.WriteString(renderInterfaceDetails(&in))
		case SectionMessageMapping:
			sb.WriteString(h2(title)); sb.WriteString(renderMessageMapping())
		case SectionEnhancementDetails:
			sb.WriteString(h2(title)); sb.WriteString(renderEnhancementDetails(&in))
		case SectionFormDetails:
			sb.WriteString(h2(title)); sb.WriteString(renderFormDetails(&in))
		case SectionWorkflowDetails:
			sb.WriteString(h2(title)); sb.WriteString(renderWorkflowDetails(&in))
		case SectionODataDetails:
			sb.WriteString(h2(title)); sb.WriteString(renderODataDetails(&in))
		case SectionPseudocode:
			sb.WriteString(h2(title)); sb.WriteString(renderPseudocode(&in))
		case SectionCallGraph:
			sb.WriteString(h2(title)); sb.WriteString(renderCallGraph(&in))
		case SectionDatabaseObjects:
			sb.WriteString(h2(title)); sb.WriteString(renderDatabaseObjects(dbs))
		case SectionCalledFunctions:
			sb.WriteString(h2(title)); sb.WriteString(renderCalledFunctions(fms, methods))
		case SectionAuthorizationObjects:
			sb.WriteString(h2(title)); sb.WriteString(renderAuthorizationObjects(authObjs))
		case SectionATCFindings:
			sb.WriteString(h2(title)); sb.WriteString(renderATCFindings(&in))
		case SectionUnitTestResults:
			sb.WriteString(h2(title)); sb.WriteString(renderUnitTestResults(&in))
		case SectionErrorHandling:
			sb.WriteString(h2(title)); sb.WriteString(renderErrorHandling(errors))
		case SectionPerformanceConsiderations:
			sb.WriteString(h2(title)); sb.WriteString(renderPerformanceConsiderations(perfIssues))
		case SectionTestScenarios:
			sb.WriteString(h2(title)); sb.WriteString(renderTestScenarios())
		case SectionAdditionalNotes:
			sb.WriteString(h2(title))
			if in.Params.Notes != "" { sb.WriteString(in.Params.Notes + "\n") } else { sb.WriteString("_No additional notes._\n") }
		case SectionChangeHistory:
			sb.WriteString(h2(title)); sb.WriteString(renderChangeHistory(&in))
		case SectionSignOff:
			sb.WriteString(h2(title)); sb.WriteString(renderSignOff(&in))
		}
	}

	return sb.String()
}
