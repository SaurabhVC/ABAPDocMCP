package dsl

import (
	"context"
	"fmt"
	"testing"
)

func TestWorkflowExecution(t *testing.T) {
	t.Run("SimpleWorkflow", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		// Register mock handler
		var searchCalled bool
		engine.RegisterHandler("mock_search", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			searchCalled = true
			return []ObjectRef{
				{Type: "CLAS", Name: "ZCL_TEST1"},
				{Type: "CLAS", Name: "ZCL_TEST2"},
			}, nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{
					Name:   "search",
					Action: "mock_search",
					SaveAs: "results",
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Errorf("expected success, got failure: %s", result.Error)
		}
		if !searchCalled {
			t.Error("mock_search handler was not called")
		}
		if len(result.StepResults) != 1 {
			t.Errorf("expected 1 step result, got %d", len(result.StepResults))
		}
		if !result.StepResults[0].Success {
			t.Error("expected step to succeed")
		}
	})

	t.Run("WorkflowWithVariables", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		var capturedQuery string
		engine.RegisterHandler("capture_search", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			capturedQuery, _ = params["query"].(string)
			return []ObjectRef{}, nil
		})

		workflow := &Workflow{
			Name: "test",
			Variables: map[string]string{
				"PACKAGE": "$TMP",
			},
			Steps: []WorkflowStep{
				{
					Action: "capture_search",
					Parameters: map[string]interface{}{
						"query": "${PACKAGE}/*",
					},
				},
			},
		}

		_, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if capturedQuery != "$TMP/*" {
			t.Errorf("expected query '$TMP/*', got '%s'", capturedQuery)
		}
	})

	t.Run("WorkflowWithOverrideVariables", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		var capturedQuery string
		engine.RegisterHandler("capture_search", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			capturedQuery, _ = params["query"].(string)
			return []ObjectRef{}, nil
		})

		workflow := &Workflow{
			Name: "test",
			Variables: map[string]string{
				"PACKAGE": "$TMP",
			},
			Steps: []WorkflowStep{
				{
					Action: "capture_search",
					Parameters: map[string]interface{}{
						"query": "${PACKAGE}/*",
					},
				},
			},
		}

		// Override variable
		_, err := engine.Execute(context.Background(), workflow,
			WithVariables(map[string]string{"PACKAGE": "$ZRAY"}))
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if capturedQuery != "$ZRAY/*" {
			t.Errorf("expected query '$ZRAY/*', got '%s'", capturedQuery)
		}
	})

	t.Run("WorkflowFailure", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		engine.RegisterHandler("failing_action", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			return nil, fmt.Errorf("simulated failure")
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{
					Name:   "will-fail",
					Action: "failing_action",
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if result.Success {
			t.Error("expected failure, got success")
		}
		if result.StepResults[0].Success {
			t.Error("expected step to fail")
		}
		if result.StepResults[0].Error != "simulated failure" {
			t.Errorf("expected error 'simulated failure', got '%s'", result.StepResults[0].Error)
		}
	})

	t.Run("WorkflowFailureContinue", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		var step2Called bool
		engine.RegisterHandler("failing_action", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			return nil, fmt.Errorf("simulated failure")
		})
		engine.RegisterHandler("success_action", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			step2Called = true
			return "ok", nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{
					Name:      "will-fail",
					Action:    "failing_action",
					OnFailure: "continue",
				},
				{
					Name:   "should-run",
					Action: "success_action",
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if !step2Called {
			t.Error("second step should have been called despite first failure")
		}
		if len(result.StepResults) != 2 {
			t.Errorf("expected 2 step results, got %d", len(result.StepResults))
		}
	})

	t.Run("WorkflowConditionSkip", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		var actionCalled bool
		engine.RegisterHandler("conditional_action", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			actionCalled = true
			return "ok", nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{
					Name:      "skipped",
					Action:    "conditional_action",
					Condition: "exists:nonexistent",
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if actionCalled {
			t.Error("action should not have been called due to condition")
		}
		if !result.StepResults[0].Skipped {
			t.Error("step should be marked as skipped")
		}
		if result.StepResults[0].SkipReason != "condition not met" {
			t.Errorf("expected skip reason 'condition not met', got '%s'", result.StepResults[0].SkipReason)
		}
	})

	t.Run("WorkflowConditionRun", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		var actionCalled bool
		engine.RegisterHandler("setup", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			return []ObjectRef{{Name: "test"}}, nil
		})
		engine.RegisterHandler("conditional_action", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			actionCalled = true
			return "ok", nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{
					Name:   "setup",
					Action: "setup",
					SaveAs: "objects",
				},
				{
					Name:      "runs",
					Action:    "conditional_action",
					Condition: "not_empty:objects",
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if !actionCalled {
			t.Error("action should have been called because condition was met")
		}
		if result.StepResults[1].Skipped {
			t.Error("step should not be marked as skipped")
		}
	})

	t.Run("UnknownAction", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{
					Name:   "unknown",
					Action: "nonexistent_action",
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if result.Success {
			t.Error("expected failure for unknown action")
		}
		if result.StepResults[0].Error != "unknown action: nonexistent_action" {
			t.Errorf("unexpected error: %s", result.StepResults[0].Error)
		}
	})

	t.Run("DryRunMode", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		var actionCalled bool
		engine.RegisterHandler("dry_test", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			actionCalled = true
			if ctx.IsDryRun() {
				return map[string]interface{}{"dryRun": true}, nil
			}
			return map[string]interface{}{"dryRun": false}, nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{Action: "dry_test", SaveAs: "result"},
			},
		}

		result, err := engine.Execute(context.Background(), workflow, WithDryRun(true))
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if !actionCalled {
			t.Error("action should be called even in dry run")
		}

		output, ok := result.Variables["result"].(map[string]interface{})
		if !ok {
			t.Fatal("expected map output")
		}
		if output["dryRun"] != true {
			t.Error("expected dryRun to be true")
		}
	})

	t.Run("SaveAsVariable", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		engine.RegisterHandler("produce", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			return []string{"item1", "item2", "item3"}, nil
		})

		var consumed interface{}
		engine.RegisterHandler("consume", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			itemsVar, _ := params["items"].(string)
			consumed, _ = ctx.Get(itemsVar)
			return nil, nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{
					Action: "produce",
					SaveAs: "myItems",
				},
				{
					Action: "consume",
					Parameters: map[string]interface{}{
						"items": "myItems",
					},
				},
			},
		}

		_, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		items, ok := consumed.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", consumed)
		}
		if len(items) != 3 {
			t.Errorf("expected 3 items, got %d", len(items))
		}
	})
}

func TestFailIfAction(t *testing.T) {
	t.Run("TestsFailedCondition", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		// Setup: create a failed test summary
		engine.RegisterHandler("mock_test", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			return &TestSummary{
				TotalTests:  10,
				PassedTests: 7,
				FailedTests: 3,
			}, nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{Action: "mock_test", SaveAs: "testResults"},
				{
					Action: "fail_if",
					Parameters: map[string]interface{}{
						"condition": "tests_failed:testResults",
						"message":   "Some tests failed!",
					},
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if result.Success {
			t.Error("expected workflow to fail due to test failures")
		}
		if result.StepResults[1].Error != "Some tests failed!" {
			t.Errorf("expected error message 'Some tests failed!', got '%s'", result.StepResults[1].Error)
		}
	})

	t.Run("TestsPassedCondition", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		// Setup: create a passing test summary
		engine.RegisterHandler("mock_test", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			return &TestSummary{
				TotalTests:  10,
				PassedTests: 10,
				FailedTests: 0,
			}, nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{Action: "mock_test", SaveAs: "testResults"},
				{
					Action: "fail_if",
					Parameters: map[string]interface{}{
						"condition": "tests_failed:testResults",
					},
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if !result.Success {
			t.Errorf("expected workflow to succeed, got error: %s", result.Error)
		}
	})

	t.Run("SyntaxErrorsCondition", func(t *testing.T) {
		engine := NewWorkflowEngine(nil)

		// Setup: create syntax check results with errors
		engine.RegisterHandler("mock_syntax", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
			return []map[string]interface{}{
				{"object": "ZCL_TEST1", "success": true},
				{"object": "ZCL_TEST2", "success": false},
			}, nil
		})

		workflow := &Workflow{
			Name: "test",
			Steps: []WorkflowStep{
				{Action: "mock_syntax", SaveAs: "syntaxResults"},
				{
					Action: "fail_if",
					Parameters: map[string]interface{}{
						"condition": "syntax_errors:syntaxResults",
					},
				},
			},
		}

		result, err := engine.Execute(context.Background(), workflow)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}

		if result.Success {
			t.Error("expected workflow to fail due to syntax errors")
		}
	})
}

func TestPrintAction(t *testing.T) {
	engine := NewWorkflowEngine(nil)

	workflow := &Workflow{
		Name: "test",
		Steps: []WorkflowStep{
			{
				Action: "print",
				Parameters: map[string]interface{}{
					"message": "Hello, World!",
				},
			},
		},
	}

	result, err := engine.Execute(context.Background(), workflow)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if !result.Success {
		t.Error("print action should succeed")
	}
}

func TestWorkflowFromYAML(t *testing.T) {
	yamlContent := `
name: integration-test
description: Full integration test workflow
variables:
  PACKAGE: "$TMP"
  MAX_RESULTS: "50"
steps:
  - name: discover
    action: mock_search
    parameters:
      query: "${PACKAGE}/*"
      maxResults: 50
    saveAs: objects

  - name: validate
    action: mock_syntax
    parameters:
      objects: objects
    saveAs: syntaxResults
    onFailure: continue

  - name: check-syntax
    action: fail_if
    parameters:
      condition: "syntax_errors:syntaxResults"
      message: "Syntax validation failed"

  - name: test
    action: mock_test
    parameters:
      objects: objects
    saveAs: testResults
    condition: "not_empty:objects"

  - name: check-tests
    action: fail_if
    parameters:
      condition: "tests_failed:testResults"
`

	engine := NewWorkflowEngine(nil)

	// Register mock handlers
	engine.RegisterHandler("mock_search", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
		return []ObjectRef{
			{Type: "CLAS", Name: "ZCL_TEST1"},
			{Type: "CLAS", Name: "ZCL_TEST2"},
		}, nil
	})

	engine.RegisterHandler("mock_syntax", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
		return []map[string]interface{}{
			{"object": "ZCL_TEST1", "success": true},
			{"object": "ZCL_TEST2", "success": true},
		}, nil
	})

	engine.RegisterHandler("mock_test", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
		return &TestSummary{
			TotalTests:  5,
			PassedTests: 5,
			FailedTests: 0,
		}, nil
	})

	// Parse and execute
	workflow, err := engine.ParseWorkflow([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflow failed: %v", err)
	}

	result, err := engine.Execute(context.Background(), workflow)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Verify
	if !result.Success {
		t.Errorf("expected success, got failure: %s", result.Error)
	}
	if len(result.StepResults) != 5 {
		t.Errorf("expected 5 step results, got %d", len(result.StepResults))
	}

	// Check each step
	expectedSteps := []string{"discover", "validate", "check-syntax", "test", "check-tests"}
	for i, expected := range expectedSteps {
		if result.StepResults[i].Name != expected {
			t.Errorf("step %d: expected name '%s', got '%s'", i, expected, result.StepResults[i].Name)
		}
		if !result.StepResults[i].Success {
			t.Errorf("step %d (%s): expected success", i, expected)
		}
	}
}

func TestWorkflowYAMLWithFailure(t *testing.T) {
	yamlContent := `
name: failing-workflow
steps:
  - action: mock_search
    saveAs: objects
  - action: mock_test
    saveAs: testResults
  - action: fail_if
    parameters:
      condition: "tests_failed:testResults"
      message: "Tests failed!"
`

	engine := NewWorkflowEngine(nil)

	engine.RegisterHandler("mock_search", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
		return []ObjectRef{{Name: "ZCL_TEST"}}, nil
	})

	engine.RegisterHandler("mock_test", func(ctx *ExecutionContext, params map[string]interface{}) (interface{}, error) {
		return &TestSummary{
			TotalTests:  3,
			PassedTests: 1,
			FailedTests: 2,
		}, nil
	})

	workflow, err := engine.ParseWorkflow([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflow failed: %v", err)
	}

	result, err := engine.Execute(context.Background(), workflow)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.Success {
		t.Error("expected workflow to fail")
	}
	if result.Error != "step 'step_3_fail_if' failed: Tests failed!" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}
