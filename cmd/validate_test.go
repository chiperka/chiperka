package cmd

import (
	"errors"
	"strings"
	"testing"

	"chiperka-cli/internal/model"
)

func intPtr(i int) *int { return &i }

func assertHasIssue(t *testing.T, issues []validationIssue, level, msgSubstr string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Level == level && strings.Contains(issue.Message, msgSubstr) {
			return
		}
	}
	t.Errorf("expected %s issue containing %q, got: %v", level, msgSubstr, issues)
}

// validBaseline returns a Test plus matching service templates and endpoint
// collection that together pass validation. Individual tests mutate the
// returned Test (or pass nil for endpoints) to exercise specific failures.
func validBaseline() (model.Test, *model.ServiceTemplateCollection, *model.EndpointCollection) {
	templates := model.NewServiceTemplateCollection()
	templates.AddTemplate(&model.ServiceTemplate{
		Name:  "api",
		Image: "nginx:alpine",
	})
	endpoints := model.NewEndpointCollection()
	endpoints.AddEndpoint(&model.Endpoint{
		Name:    "api-root",
		Service: "api",
		HTTP:    &model.EndpointHTTP{Method: "GET", URL: "/"},
	})
	test := model.Test{
		Name:     "valid-test",
		Endpoint: "api-root",
		Services: []model.Service{{Ref: "api"}},
		Execution: model.Execution{
			Executor: model.ExecutorHTTP,
			Target:   "http://api",
			Request:  model.HTTPRequest{Method: "GET", URL: "/"},
		},
		Assertions: []model.Assertion{
			{Response: &model.ResponseAssertion{StatusCode: intPtr(200)}},
		},
	}
	return test, templates, endpoints
}

func TestValidateTest_Valid(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()

	issues := validateTest(test, suite, templates, endpoints)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d: %v", len(issues), issues)
	}
}

func TestValidateTest_NoServices(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "no-services"
	test.Services = nil

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "no services defined")
}

func TestValidateTest_RejectsInlineService(t *testing.T) {
	// Inline service definitions are no longer supported. Every service entry
	// must point at a kind: Service via Ref.
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "inline-service"
	test.Services = []model.Service{{Name: "api"}} // no Ref

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "missing required 'ref'")
}

func TestValidateTest_BrokenRef(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "broken-ref"
	test.Services = []model.Service{{Ref: "nonexistent"}}

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "not found")
}

func TestValidateTest_RefEmptyImage(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, _, endpoints := validBaseline()
	test.Name = "ref-no-image"
	test.Services = []model.Service{{Ref: "broken"}}

	templates := model.NewServiceTemplateCollection()
	templates.AddTemplate(&model.ServiceTemplate{
		Name: "broken",
		// Image intentionally empty
	})

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "image is empty after resolving")
}

func TestValidateTest_MissingTarget(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "no-target"
	test.Execution = model.Execution{
		Executor: model.ExecutorHTTP,
		Request:  model.HTTPRequest{Method: "GET", URL: "/"},
	}

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "target is empty")
}

func TestValidateTest_MissingMethod(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "no-method"
	test.Execution = model.Execution{
		Executor: model.ExecutorHTTP,
		Target:   "http://api",
	}

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "method is empty")
}

func TestValidateTest_MissingEndpointReference(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "no-endpoint"
	test.Endpoint = ""

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "spec.endpoint is empty")
}

func TestValidateTest_UnknownEndpointReference(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "unknown-endpoint"
	test.Endpoint = "does-not-exist"

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "unknown kind: Endpoint")
}

func TestValidateTest_CLIExecutor(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "valid-cli"
	test.Execution = model.Execution{
		Executor: model.ExecutorCLI,
		CLI: &model.CLICommand{
			Service: "api",
			Command: "echo hello",
		},
	}
	test.Assertions = []model.Assertion{
		{CLI: &model.CLIAssertion{ExitCode: intPtr(0)}},
	}

	issues := validateTest(test, suite, templates, endpoints)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d: %v", len(issues), issues)
	}
}

func TestValidateTest_CLIMissingConfig(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "cli-no-config"
	test.Execution = model.Execution{Executor: model.ExecutorCLI}
	test.Assertions = []model.Assertion{
		{CLI: &model.CLIAssertion{ExitCode: intPtr(0)}},
	}

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "cli executor requires cli configuration")
}

func TestValidateTest_CLIMissingService(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "cli-no-service"
	test.Execution = model.Execution{
		Executor: model.ExecutorCLI,
		CLI:      &model.CLICommand{Command: "echo hello"},
	}
	test.Assertions = []model.Assertion{
		{CLI: &model.CLIAssertion{ExitCode: intPtr(0)}},
	}

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "cli.service is empty")
}

func TestValidateTest_CLIMissingCommand(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "cli-no-command"
	test.Execution = model.Execution{
		Executor: model.ExecutorCLI,
		CLI:      &model.CLICommand{Service: "api"},
	}
	test.Assertions = []model.Assertion{
		{CLI: &model.CLIAssertion{ExitCode: intPtr(0)}},
	}

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "cli.command is empty")
}

func TestValidateTest_UnknownExecutor(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "bad-executor"
	test.Execution = model.Execution{Executor: "grpc"}

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "unknown executor type")
}

func TestValidateTest_NoAssertionsWarning(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = "no-assertions"
	test.Assertions = nil

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "warning", "no assertions defined")
	// Should be warning only, no errors
	for _, issue := range issues {
		if issue.Level == "error" {
			t.Errorf("unexpected error: %s", issue.Message)
		}
	}
}

func TestValidateTest_EmptyName(t *testing.T) {
	suite := model.Suite{Name: "Suite", FilePath: "test.chiperka"}
	test, templates, endpoints := validBaseline()
	test.Name = ""

	issues := validateTest(test, suite, templates, endpoints)
	assertHasIssue(t, issues, "error", "metadata.name is empty")
}

func TestExitError(t *testing.T) {
	err := exitErrorf(ExitTestFailure, "test failed: %d errors", 3)

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatal("expected ExitError")
	}
	if exitErr.Code != ExitTestFailure {
		t.Errorf("expected code %d, got %d", ExitTestFailure, exitErr.Code)
	}
	if exitErr.Error() != "test failed: 3 errors" {
		t.Errorf("unexpected message: %s", exitErr.Error())
	}
}
