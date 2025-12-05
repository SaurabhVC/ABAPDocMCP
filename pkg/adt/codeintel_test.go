package adt

import (
	"testing"
)

func TestParseClassComponents(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<abapsource:objectStructureElement xmlns:abapsource="http://www.sap.com/adt/abapsource"
    xmlns:adtcore="http://www.sap.com/adt/core"
    xmlns:atom="http://www.w3.org/2005/Atom"
    adtcore:name="ZCL_TEST" adtcore:type="CLAS/OC" visibility="public">
  <atom:link href="/sap/bc/adt/oo/classes/ZCL_TEST" rel="self" type="application/xml"/>
  <abapsource:objectStructureElement adtcore:name="CONSTRUCTOR" adtcore:type="CLAS/OM"
      visibility="public" isStatic="false" isFinal="false" isAbstract="false"
      description="Constructor">
    <atom:link href="/sap/bc/adt/oo/classes/ZCL_TEST/methods/CONSTRUCTOR" rel="self"/>
  </abapsource:objectStructureElement>
  <abapsource:objectStructureElement adtcore:name="PROCESS_DATA" adtcore:type="CLAS/OM"
      visibility="public" isStatic="false" isFinal="true" isAbstract="false"
      description="Process data method">
    <atom:link href="/sap/bc/adt/oo/classes/ZCL_TEST/methods/PROCESS_DATA" rel="self"/>
  </abapsource:objectStructureElement>
  <abapsource:objectStructureElement adtcore:name="HELPER" adtcore:type="CLAS/OM"
      visibility="private" isStatic="true" isFinal="false" isAbstract="false">
    <atom:link href="/sap/bc/adt/oo/classes/ZCL_TEST/methods/HELPER" rel="self"/>
  </abapsource:objectStructureElement>
  <abapsource:objectStructureElement adtcore:name="MV_DATA" adtcore:type="CLAS/OD"
      visibility="private" readOnly="false">
    <atom:link href="/sap/bc/adt/oo/classes/ZCL_TEST/attributes/MV_DATA" rel="self"/>
  </abapsource:objectStructureElement>
  <abapsource:objectStructureElement adtcore:name="MC_CONSTANT" adtcore:type="CLAS/OD"
      visibility="public" constant="true" readOnly="true">
    <atom:link href="/sap/bc/adt/oo/classes/ZCL_TEST/attributes/MC_CONSTANT" rel="self"/>
  </abapsource:objectStructureElement>
  <abapsource:objectStructureElement adtcore:name="ON_CHANGE" adtcore:type="CLAS/OE"
      visibility="public">
    <atom:link href="/sap/bc/adt/oo/classes/ZCL_TEST/events/ON_CHANGE" rel="self"/>
  </abapsource:objectStructureElement>
</abapsource:objectStructureElement>`

	result, err := parseClassComponents([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseClassComponents failed: %v", err)
	}

	// Check root element
	if result.Name != "ZCL_TEST" {
		t.Errorf("expected name 'ZCL_TEST', got '%s'", result.Name)
	}
	if result.Type != "CLAS/OC" {
		t.Errorf("expected type 'CLAS/OC', got '%s'", result.Type)
	}
	if result.Visibility != "public" {
		t.Errorf("expected visibility 'public', got '%s'", result.Visibility)
	}

	// Check components count
	if len(result.Components) != 6 {
		t.Fatalf("expected 6 components, got %d", len(result.Components))
	}

	// Check method with description
	constructor := result.Components[0]
	if constructor.Name != "CONSTRUCTOR" {
		t.Errorf("expected first component name 'CONSTRUCTOR', got '%s'", constructor.Name)
	}
	if constructor.Description != "Constructor" {
		t.Errorf("expected description 'Constructor', got '%s'", constructor.Description)
	}

	// Check final method
	processData := result.Components[1]
	if !processData.IsFinal {
		t.Error("expected PROCESS_DATA to be final")
	}

	// Check static method
	helper := result.Components[2]
	if !helper.IsStatic {
		t.Error("expected HELPER to be static")
	}
	if helper.Visibility != "private" {
		t.Errorf("expected HELPER visibility 'private', got '%s'", helper.Visibility)
	}

	// Check constant attribute
	constant := result.Components[4]
	if !constant.Constant {
		t.Error("expected MC_CONSTANT to be constant")
	}
	if !constant.ReadOnly {
		t.Error("expected MC_CONSTANT to be read-only")
	}

	// Check event
	event := result.Components[5]
	if event.Name != "ON_CHANGE" {
		t.Errorf("expected event name 'ON_CHANGE', got '%s'", event.Name)
	}
}

func TestParseClassComponentsEmpty(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<abapsource:objectStructureElement xmlns:abapsource="http://www.sap.com/adt/abapsource"
    xmlns:adtcore="http://www.sap.com/adt/core"
    adtcore:name="ZCL_EMPTY" adtcore:type="CLAS/OC" visibility="public">
</abapsource:objectStructureElement>`

	result, err := parseClassComponents([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseClassComponents failed: %v", err)
	}

	if result.Name != "ZCL_EMPTY" {
		t.Errorf("expected name 'ZCL_EMPTY', got '%s'", result.Name)
	}
	if len(result.Components) != 0 {
		t.Errorf("expected 0 components, got %d", len(result.Components))
	}
}

func TestParseClassComponentsNested(t *testing.T) {
	// Test nested components (e.g., local types within methods)
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<abapsource:objectStructureElement xmlns:abapsource="http://www.sap.com/adt/abapsource"
    xmlns:adtcore="http://www.sap.com/adt/core"
    adtcore:name="ZCL_NESTED" adtcore:type="CLAS/OC" visibility="public">
  <abapsource:objectStructureElement adtcore:name="OUTER_METHOD" adtcore:type="CLAS/OM" visibility="public">
    <abapsource:objectStructureElement adtcore:name="LT_LOCAL" adtcore:type="CLAS/OT" visibility="private">
    </abapsource:objectStructureElement>
  </abapsource:objectStructureElement>
</abapsource:objectStructureElement>`

	result, err := parseClassComponents([]byte(xmlData))
	if err != nil {
		t.Fatalf("parseClassComponents failed: %v", err)
	}

	if len(result.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(result.Components))
	}

	outerMethod := result.Components[0]
	if outerMethod.Name != "OUTER_METHOD" {
		t.Errorf("expected outer method name 'OUTER_METHOD', got '%s'", outerMethod.Name)
	}

	// Check nested component
	if len(outerMethod.Components) != 1 {
		t.Fatalf("expected 1 nested component, got %d", len(outerMethod.Components))
	}

	localType := outerMethod.Components[0]
	if localType.Name != "LT_LOCAL" {
		t.Errorf("expected local type name 'LT_LOCAL', got '%s'", localType.Name)
	}
}
