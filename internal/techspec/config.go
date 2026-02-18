// Package techspec provides WRICEF Technical Specification generation for ABAPDocMCP.
package techspec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// SectionID identifies a section in the tech spec document.
type SectionID = string

// All available section IDs.
const (
	SectionDocumentInfo             SectionID = "documentInfo"
	SectionDeveloperInfo            SectionID = "developerInfo"
	SectionChangeRequest            SectionID = "changeRequest"
	SectionWricefClassification     SectionID = "wricefClassification"
	SectionTransportDetails         SectionID = "transportDetails"
	SectionTransportContents        SectionID = "transportContents"
	SectionBusinessRequirement      SectionID = "businessRequirement"
	SectionScopeAssumptions         SectionID = "scopeAssumptions"
	SectionTechnicalDesign          SectionID = "technicalDesign"
	SectionObjectStructure          SectionID = "objectStructure"
	SectionTypeHierarchy            SectionID = "typeHierarchy"
	SectionObjectRelationDiagram    SectionID = "objectRelationDiagram"
	SectionReportDetails            SectionID = "reportDetails"
	SectionInterfaceDetails         SectionID = "interfaceDetails"
	SectionMessageMapping           SectionID = "messageMapping"
	SectionEnhancementDetails       SectionID = "enhancementDetails"
	SectionFormDetails              SectionID = "formDetails"
	SectionWorkflowDetails          SectionID = "workflowDetails"
	SectionODataDetails             SectionID = "odataDetails"
	SectionPseudocode               SectionID = "pseudocode"
	SectionCallGraph                SectionID = "callGraph"
	SectionDatabaseObjects          SectionID = "databaseObjects"
	SectionCalledFunctions          SectionID = "calledFunctions"
	SectionAuthorizationObjects     SectionID = "authorizationObjects"
	SectionATCFindings              SectionID = "atcFindings"
	SectionUnitTestResults          SectionID = "unitTestResults"
	SectionErrorHandling            SectionID = "errorHandling"
	SectionPerformanceConsiderations SectionID = "performanceConsiderations"
	SectionTestScenarios            SectionID = "testScenarios"
	SectionAdditionalNotes          SectionID = "additionalNotes"
	SectionChangeHistory            SectionID = "changeHistory"
	SectionSignOff                  SectionID = "signOff"
)

// DefaultSections is the default ordered list of sections.
var DefaultSections = []SectionID{
	SectionDocumentInfo,
	SectionDeveloperInfo,
	SectionChangeRequest,
	SectionWricefClassification,
	SectionTransportDetails,
	SectionTransportContents,
	SectionBusinessRequirement,
	SectionScopeAssumptions,
	SectionTechnicalDesign,
	SectionObjectStructure,
	SectionTypeHierarchy,
	SectionObjectRelationDiagram,
	SectionReportDetails,
	SectionInterfaceDetails,
	SectionMessageMapping,
	SectionEnhancementDetails,
	SectionFormDetails,
	SectionWorkflowDetails,
	SectionODataDetails,
	SectionPseudocode,
	SectionCallGraph,
	SectionDatabaseObjects,
	SectionCalledFunctions,
	SectionAuthorizationObjects,
	SectionATCFindings,
	SectionUnitTestResults,
	SectionErrorHandling,
	SectionPerformanceConsiderations,
	SectionTestScenarios,
	SectionAdditionalNotes,
	SectionChangeHistory,
	SectionSignOff,
}

// DefaultTitles maps section IDs to their default heading text.
var DefaultTitles = map[SectionID]string{
	SectionDocumentInfo:              "Document Information",
	SectionDeveloperInfo:             "Developer Information",
	SectionChangeRequest:             "Change Request Reference",
	SectionWricefClassification:      "WRICEF Classification",
	SectionTransportDetails:          "Transport Request Details",
	SectionTransportContents:         "Transport Contents",
	SectionBusinessRequirement:       "Business Requirement",
	SectionScopeAssumptions:          "Scope & Assumptions",
	SectionTechnicalDesign:           "Technical Design Overview",
	SectionObjectStructure:           "Object Structure",
	SectionTypeHierarchy:             "Type Hierarchy",
	SectionObjectRelationDiagram:     "Object Relationship Diagram",
	SectionReportDetails:             "Report Details",
	SectionInterfaceDetails:          "Interface Details",
	SectionMessageMapping:            "Message / Field Mapping",
	SectionEnhancementDetails:        "Enhancement Details",
	SectionFormDetails:               "Form Details",
	SectionWorkflowDetails:           "Workflow Details",
	SectionODataDetails:              "OData / RAP Service Details",
	SectionPseudocode:                "Processing Logic (Pseudocode)",
	SectionCallGraph:                 "Call Graph",
	SectionDatabaseObjects:           "Database Objects Used",
	SectionCalledFunctions:           "Called Function Modules / BAPIs / Methods",
	SectionAuthorizationObjects:      "Authorization Objects",
	SectionATCFindings:               "ATC Code Quality Findings",
	SectionUnitTestResults:           "Unit Test Results",
	SectionErrorHandling:             "Error Handling",
	SectionPerformanceConsiderations: "Performance Considerations",
	SectionTestScenarios:             "Test Scenarios",
	SectionAdditionalNotes:           "Additional Notes",
	SectionChangeHistory:             "Change History",
	SectionSignOff:                   "Sign-off",
}

// FileConfig mirrors the JSON structure of tech-spec-config.json.
type FileConfig struct {
	Sections      []SectionID          `json:"sections"`
	SectionTitles map[SectionID]string `json:"sectionTitles"`
}

var (
	cfgOnce   sync.Once
	cfgCached *FileConfig
)

// LoadConfig loads tech-spec-config.json from the working directory or next to the binary.
// Returns nil (use defaults) if the file is not found.
func LoadConfig() *FileConfig {
	cfgOnce.Do(func() {
		candidates := []string{
			filepath.Join(".", "tech-spec-config.json"),
		}
		// Also check next to the binary
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "tech-spec-config.json"))
		}
		// And next to this source file (for `go run` dev mode)
		_, thisFile, _, ok := runtime.Caller(0)
		if ok {
			candidates = append(candidates, filepath.Join(filepath.Dir(thisFile), "..", "..", "tech-spec-config.json"))
		}

		for _, path := range candidates {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var cfg FileConfig
			if err := json.Unmarshal(data, &cfg); err == nil {
				cfgCached = &cfg
				return
			}
		}
	})
	return cfgCached
}

// ResolvedSections returns the ordered section list from config, falling back to defaults.
func ResolvedSections() []SectionID {
	cfg := LoadConfig()
	if cfg != nil && len(cfg.Sections) > 0 {
		return cfg.Sections
	}
	return DefaultSections
}

// ResolvedTitle returns the heading for a section from config, falling back to defaults.
func ResolvedTitle(id SectionID) string {
	cfg := LoadConfig()
	if cfg != nil {
		if title, ok := cfg.SectionTitles[id]; ok && title != "" {
			return title
		}
	}
	if title, ok := DefaultTitles[id]; ok {
		return title
	}
	return id
}
