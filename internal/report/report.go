// Package report provides report generation, storage, and retrieval.
//
// Reports are generated from test results or specification data and stored
// under .chiperka/reports/ organized by scope:
//
//   .chiperka/reports/run/<run-uuid>/<report-type>/
//   .chiperka/reports/test/<test-uuid>/<report-type>/
//   .chiperka/reports/global/<report-type>/
//
// Report types and their resolvers are configured in chiperka.yaml under the
// reports: key. Built-in resolvers use the "chiperka." prefix.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chiperka-cli/internal/config"
	"chiperka-cli/internal/model"
	"chiperka-cli/internal/output"
	"chiperka-cli/internal/result"
)

const (
	// ScopeRun generates a report for an entire test run.
	ScopeRun = "run"
	// ScopeTest generates a report for a single test.
	ScopeTest = "test"
	// ScopeGlobal generates a report from specification data (no run required).
	ScopeGlobal = "global"
)

const baseDir = ".chiperka/reports"

// ReportMeta is metadata about a generated report, stored as meta.json.
type ReportMeta struct {
	Type        string    `json:"type"`
	Scope       string    `json:"scope"`
	ScopeID     string    `json:"scope_id,omitempty"` // run or test UUID
	GeneratedAt time.Time `json:"generated_at"`
	Resolver    string    `json:"resolver"`
	Files       []string  `json:"files"`
}

// ReportRef is a compact reference returned by List.
type ReportRef struct {
	Type        string    `json:"type"`
	Scope       string    `json:"scope"`
	ScopeID     string    `json:"scope_id,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
	Path        string    `json:"path"`
}

// AvailableType describes a report type that can be generated.
type AvailableType struct {
	Type     string   `json:"type"`
	Scopes   []string `json:"scopes"`
	Resolver string   `json:"resolver"`
}

// AvailableTypes returns report types configured in chiperka.yaml.
func AvailableTypes(cfg *config.Config) []AvailableType {
	if cfg == nil || len(cfg.Reports) == 0 {
		return []AvailableType{}
	}
	var types []AvailableType
	for name, rc := range cfg.Reports {
		types = append(types, AvailableType{
			Type:     name,
			Scopes:   rc.On,
			Resolver: rc.Resolver,
		})
	}
	return types
}

// reportDir returns the directory where a report is stored.
func reportDir(scope, scopeID, reportType string) string {
	switch scope {
	case ScopeGlobal:
		return filepath.Join(baseDir, "global", reportType)
	default:
		return filepath.Join(baseDir, scope, scopeID, reportType)
	}
}

// Generate creates a report of the given type for the given scope.
func Generate(cfg *config.Config, reportType, scope, scopeID string) (*ReportMeta, error) {
	if cfg == nil || cfg.Reports == nil {
		return nil, fmt.Errorf("no reports configured in chiperka.yaml")
	}

	rc, ok := cfg.Reports[reportType]
	if !ok {
		return nil, fmt.Errorf("unknown report type %q (not configured in chiperka.yaml)", reportType)
	}

	if !scopeAllowed(rc, scope) {
		return nil, fmt.Errorf("report %q does not support scope %q (configured: %v)", reportType, scope, rc.On)
	}

	if scope != ScopeGlobal && scopeID == "" {
		return nil, fmt.Errorf("scope %q requires a UUID", scope)
	}

	dir := reportDir(scope, scopeID, reportType)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create report directory: %w", err)
	}

	var err error
	if strings.HasPrefix(rc.Resolver, "chiperka.") {
		err = runBuiltinResolver(rc.Resolver, scope, scopeID, dir)
	} else {
		err = runCustomResolver(rc.Resolver, scope, scopeID, dir)
	}
	if err != nil {
		return nil, fmt.Errorf("resolver %q failed: %w", rc.Resolver, err)
	}

	files, err := listFiles(dir)
	if err != nil {
		return nil, err
	}

	meta := &ReportMeta{
		Type:        reportType,
		Scope:       scope,
		ScopeID:     scopeID,
		GeneratedAt: time.Now(),
		Resolver:    rc.Resolver,
		Files:       files,
	}

	metaPath := filepath.Join(dir, "meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return nil, err
	}

	return meta, nil
}

// GenerateFromResult generates a report directly from a RunResult (used during test execution).
func GenerateFromResult(cfg *config.Config, reportType, scope, scopeID string, runResult *model.RunResult, version string) (*ReportMeta, error) {
	if cfg == nil || cfg.Reports == nil {
		return nil, fmt.Errorf("no reports configured in chiperka.yaml")
	}

	rc, ok := cfg.Reports[reportType]
	if !ok {
		return nil, fmt.Errorf("unknown report type %q", reportType)
	}

	if !scopeAllowed(rc, scope) {
		return nil, fmt.Errorf("report %q does not support scope %q", reportType, scope)
	}

	dir := reportDir(scope, scopeID, reportType)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create report directory: %w", err)
	}

	switch rc.Resolver {
	case "chiperka.html-reporter":
		hw := output.NewHTMLWriter()
		if scope == ScopeTest {
			// Find the specific test in run result
			for _, sr := range runResult.SuiteResults {
				for i := range sr.TestResults {
					tr := &sr.TestResults[i]
					if tr.UUID == scopeID {
						if _, err := hw.WriteTestReport(tr, sr.Suite.Name, sr.Suite.FilePath, dir); err != nil {
							return nil, err
						}
					}
				}
			}
		} else {
			if err := hw.WriteDashboard(runResult, dir, version); err != nil {
				return nil, err
			}
		}
	case "chiperka.junit-reporter":
		jw := output.NewJUnitWriter()
		if err := jw.Write(runResult, filepath.Join(dir, "report.xml")); err != nil {
			return nil, err
		}
	default:
		// Custom resolver — pass result as JSON on stdin
		return nil, fmt.Errorf("custom resolvers not supported during test execution (use 'chiperka report generate' after the run)")
	}

	files, err := listFiles(dir)
	if err != nil {
		return nil, err
	}

	meta := &ReportMeta{
		Type:        reportType,
		Scope:       scope,
		ScopeID:     scopeID,
		GeneratedAt: time.Now(),
		Resolver:    rc.Resolver,
		Files:       files,
	}

	metaPath := filepath.Join(dir, "meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return nil, err
	}

	return meta, nil
}

// List returns all generated reports, optionally filtered by scope and scopeID.
func List(scope, scopeID string) ([]ReportRef, error) {
	var refs []ReportRef

	if scope != "" {
		// List reports for a specific scope
		var searchDir string
		if scope == ScopeGlobal {
			searchDir = filepath.Join(baseDir, "global")
		} else if scopeID != "" {
			searchDir = filepath.Join(baseDir, scope, scopeID)
		} else {
			searchDir = filepath.Join(baseDir, scope)
		}
		found, err := findReportsIn(searchDir, scope)
		if err != nil {
			return nil, err
		}
		refs = append(refs, found...)
	} else {
		// List all reports
		for _, s := range []string{ScopeRun, ScopeTest, ScopeGlobal} {
			dir := filepath.Join(baseDir, s)
			if s == ScopeGlobal {
				dir = filepath.Join(baseDir, "global")
			}
			found, err := findReportsIn(dir, s)
			if err != nil {
				continue
			}
			refs = append(refs, found...)
		}
	}

	if refs == nil {
		refs = []ReportRef{}
	}
	return refs, nil
}

// Get reads a generated report's metadata.
func Get(reportType, scope, scopeID string) (*ReportMeta, error) {
	dir := reportDir(scope, scopeID, reportType)
	metaPath := filepath.Join(dir, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("report %q not found for %s %s", reportType, scope, scopeID)
		}
		return nil, err
	}

	var meta ReportMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetFile reads a single file from a generated report.
func GetFile(reportType, scope, scopeID, fileName string) ([]byte, error) {
	dir := reportDir(scope, scopeID, reportType)
	path := filepath.Join(dir, fileName)

	// Prevent path traversal
	absDir, _ := filepath.Abs(dir)
	absPath, _ := filepath.Abs(path)
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		return nil, fmt.Errorf("invalid file path")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file %q not found in report %q", fileName, reportType)
		}
		return nil, err
	}
	return content, nil
}

// --- Built-in resolvers ---

func runBuiltinResolver(resolver, scope, scopeID, outputDir string) error {
	switch resolver {
	case "chiperka.html-reporter":
		return runHTMLResolver(scope, scopeID, outputDir)
	case "chiperka.junit-reporter":
		return runJUnitResolver(scope, scopeID, outputDir)
	default:
		return fmt.Errorf("unknown built-in resolver %q", resolver)
	}
}

func runHTMLResolver(scope, scopeID, outputDir string) error {
	store := result.DefaultLocalStore()
	hw := output.NewHTMLWriter()

	switch scope {
	case ScopeRun:
		run, err := store.GetRun(scopeID)
		if err != nil {
			return err
		}
		// Write individual test reports
		for _, testRef := range run.Tests {
			testDetail, err := store.GetTest(testRef.UUID)
			if err != nil {
				continue
			}
			testResult := detailToTestResult(testDetail)
			if _, err := hw.WriteTestReport(testResult, testDetail.Suite, "", outputDir); err != nil {
				return fmt.Errorf("write test report %s: %w", testRef.Name, err)
			}
		}
		// Write dashboard from stored result
		runResult := buildRunResultFromStore(store, run)
		if err := hw.WriteDashboard(runResult, outputDir, ""); err != nil {
			return err
		}
		return nil

	case ScopeTest:
		testDetail, err := store.GetTest(scopeID)
		if err != nil {
			return err
		}
		testResult := detailToTestResult(testDetail)
		_, err = hw.WriteTestReport(testResult, testDetail.Suite, "", outputDir)
		return err

	default:
		return fmt.Errorf("html reporter does not support scope %q", scope)
	}
}

func runJUnitResolver(scope, scopeID, outputDir string) error {
	if scope != ScopeRun {
		return fmt.Errorf("junit reporter only supports run scope")
	}

	store := result.DefaultLocalStore()
	run, err := store.GetRun(scopeID)
	if err != nil {
		return err
	}

	runResult := buildRunResultFromStore(store, run)
	jw := output.NewJUnitWriter()
	return jw.Write(runResult, filepath.Join(outputDir, "report.xml"))
}

// --- Custom resolver ---

func runCustomResolver(command, scope, scopeID, outputDir string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append(os.Environ(),
		"CHIPERKA_REPORT_SCOPE="+scope,
		"CHIPERKA_REPORT_SCOPE_ID="+scopeID,
		"CHIPERKA_REPORT_OUTPUT_DIR="+outputDir,
	)

	// If run-scoped, pass result data on stdin
	if scope == ScopeRun || scope == ScopeTest {
		store := result.DefaultLocalStore()
		if scope == ScopeRun {
			run, err := store.GetRun(scopeID)
			if err != nil {
				return err
			}
			data, _ := json.Marshal(run)
			cmd.Stdin = strings.NewReader(string(data))
		} else {
			test, err := store.GetTest(scopeID)
			if err != nil {
				return err
			}
			data, _ := json.Marshal(test)
			cmd.Stdin = strings.NewReader(string(data))
		}
	}

	cmd.Stdout = os.Stderr // resolver stdout goes to stderr so it doesn't mix with CLI output
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// --- Helpers ---

func scopeAllowed(rc *config.ReportConfig, scope string) bool {
	for _, s := range rc.On {
		if s == scope {
			return true
		}
	}
	return false
}

func listFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Name() == "meta.json" {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, rel)
		return nil
	})
	return files, err
}

func findReportsIn(dir, scope string) ([]ReportRef, error) {
	var refs []ReportRef

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.Name() != "meta.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var meta ReportMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil
		}

		refs = append(refs, ReportRef{
			Type:        meta.Type,
			Scope:       meta.Scope,
			ScopeID:     meta.ScopeID,
			GeneratedAt: meta.GeneratedAt,
			Path:        filepath.Dir(path),
		})
		return nil
	})

	return refs, err
}

// detailToTestResult converts a stored TestDetail back to a model.TestResult
// with enough data for report generation.
func detailToTestResult(detail *result.TestDetail) *model.TestResult {
	tr := &model.TestResult{
		Test: model.Test{
			Name: detail.Name,
		},
		Status:   model.TestStatus(detail.Status),
		Duration: time.Duration(detail.Duration) * time.Millisecond,
		UUID:     detail.UUID,
	}

	for _, a := range detail.Assertions {
		tr.AssertionResults = append(tr.AssertionResults, model.AssertionResult{
			Passed:   a.Passed,
			Type:     a.Type,
			Expected: a.Expected,
			Actual:   a.Actual,
			Message:  a.Message,
		})
	}

	for _, h := range detail.HTTPExchanges {
		exchange := model.HTTPExchangeResult{
			Phase:              h.Phase,
			PhaseSeq:           h.Sequence,
			RequestMethod:      h.Method,
			RequestURL:         h.URL,
			RequestHeaders:     h.RequestHeaders,
			RequestBody:        h.RequestBody,
			ResponseStatusCode: h.StatusCode,
			ResponseBody:       h.ResponseBody,
			Duration:           time.Duration(h.Duration) * time.Millisecond,
		}
		tr.HTTPExchanges = append(tr.HTTPExchanges, exchange)
	}

	for _, c := range detail.CLIExecutions {
		execution := model.CLIExecutionResult{
			Phase:      c.Phase,
			PhaseSeq:   c.Sequence,
			Service:    c.Service,
			Command:    c.Command,
			WorkingDir: c.WorkingDir,
			ExitCode:   c.ExitCode,
			Stdout:     c.Stdout,
			Stderr:     c.Stderr,
			Duration:   time.Duration(c.Duration) * time.Millisecond,
		}
		tr.CLIExecutions = append(tr.CLIExecutions, execution)
	}

	for _, s := range detail.Services {
		tr.ServiceResults = append(tr.ServiceResults, model.ServiceResult{
			Name:     s.Name,
			Image:    s.Image,
			Duration: time.Duration(s.Duration) * time.Millisecond,
		})
	}

	return tr
}

// buildRunResultFromStore reconstructs a model.RunResult from stored data.
func buildRunResultFromStore(store result.Store, run *result.RunSummary) *model.RunResult {
	runResult := &model.RunResult{}

	// Group tests by suite
	suiteTests := make(map[string][]model.TestResult)
	suiteNames := make(map[string]bool)

	for _, testRef := range run.Tests {
		detail, err := store.GetTest(testRef.UUID)
		if err != nil {
			continue
		}
		tr := detailToTestResult(detail)
		suiteTests[detail.Suite] = append(suiteTests[detail.Suite], *tr)
		suiteNames[detail.Suite] = true
	}

	for suiteName := range suiteNames {
		sr := model.SuiteResult{
			Suite: model.Suite{
				Name: suiteName,
			},
			TestResults: suiteTests[suiteName],
		}
		runResult.SuiteResults = append(runResult.SuiteResults, sr)
	}

	return runResult
}
