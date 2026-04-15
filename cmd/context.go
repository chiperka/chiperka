package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print AI-readable tool reference",
	Long: `Context outputs a markdown reference for AI agents and LLM tools.

Pipe it into your AI agent's context, project instructions, or save as a file:
  chiperka context >> CLAUDE.md
  chiperka context > .cursorrules
  chiperka context > AGENTS.md`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(strings.ReplaceAll(ContextText, "{{VERSION}}", Version))
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

const ContextText = `# Chiperka Test Runner ({{VERSION}})

Chiperka runs integration tests in isolated Docker containers.
Resources are defined in ` + "`" + `.chiperka` + "`" + ` YAML files with a top-level ` + "`" + `kind:` + "`" + ` field.

## Core concepts

There are three resource kinds:

- **endpoint** — declares a callable entry point on a service (method, URL, inputs).
  Endpoints describe *what can be called* but carry no concrete data.
- **test** — a concrete invocation of an endpoint with data and optional assertions.
  A test without assertions is a use-case (runnable via play button / MCP).
- **service** — a reusable Docker service template (image, healthcheck, env).

The relationship: endpoint references a service, test references an endpoint (or
calls a service directly). This lets you discover what exists, what is tested,
and what is missing.

## Commands

### chiperka test [path]
Run tests. Exit codes: 0=passed, 1=assertion failures, 2=infrastructure errors.

Key flags:
  --json              NDJSON output for machine consumption
  --filter "pattern"  Run tests matching name pattern (supports * wildcard)
  --tags smoke,api    Run tests with specified tags
  --configuration f   Path to chiperka.yaml config file
  --report=html       Generate report after run (key from chiperka.yaml reports)
  --report=junit      Can be specified multiple times
  --verbose           Detailed log output
  --timeout N         Seconds per test (default 300)

### chiperka validate [path]
Validate test files without executing. Exit codes: 0=valid, 1=error, 3=validation errors.

### chiperka list <kind>
List all resources of a given kind. Reads .chiperka files from discovery paths.

  chiperka list test        List all tests (name, suite, tags, file)
  chiperka list service     List all service templates (name, image, file)
  chiperka list endpoint    List all endpoints (name, service, method, url, file)
  --json                    JSON output

### chiperka get <kind> <name>
Show full detail of a single resource by name.

  chiperka get test <name>       Full test detail (services, execution, assertions)
  chiperka get service <name>    Full service template detail
  chiperka get endpoint <name>   Full endpoint detail with inputs
  --json                         JSON output

### chiperka report types
Show available report types configured in chiperka.yaml.

### chiperka report generate <type> [--run <uuid>] [--test <uuid>]
Generate a report. Scope is inferred: --run for run-scoped, --test for test-scoped,
neither for global-scoped reports.

### chiperka report list [--scope run] [--scope-id <uuid>]
List generated reports on disk. Optionally filter by scope.

### chiperka report get <type> [--run <uuid>] [--file <name>]
Read a generated report's metadata, or a specific file from it with --file.

### chiperka result runs / run / test / artifact
Inspect stored test results (progressive disclosure — see workflow below).

## Resource file formats (.chiperka)

### Endpoint

` + "```" + `yaml
kind: endpoint
name: register-user
service: api
method: POST
url: /api/register
inputs:
  - name: email
    type: string
    required: true
  - name: password
    type: string
    required: true
` + "```" + `

### Test

` + "```" + `yaml
kind: test
name: auth-suite
tests:
  - name: register-new-user
    tags: [smoke, auth]
    services:
      - name: api
        image: myapp:latest
        healthcheck:
          test: "curl -f http://localhost:8080/health"
          retries: 30
      - ref: postgres
        environment:
          POSTGRES_DB: testdb
    setup:
      - http:
          target: http://api:8080
          request:
            method: POST
            url: /seed
    execution:
      target: http://api:8080
      request:
        method: POST
        url: /api/register
        headers:
          Content-Type: application/json
        body: '{"email": "test@example.com", "password": "secret"}'
    assertions:
      - response:
          statusCode: 201
    teardown:
      - http:
          target: http://api:8080
          request:
            method: POST
            url: /cleanup
` + "```" + `

### Service template

` + "```" + `yaml
kind: service
name: postgres
image: postgres:15
healthcheck:
  test: "pg_isready"
  retries: 30
environment:
  POSTGRES_PASSWORD: test
` + "```" + `

## Configuration (.chiperka/chiperka.yaml)

` + "```" + `yaml
discovery:
  - tests/
  - services/
  - endpoints/

reports:
  html:
    on: [test, run]
    resolver: chiperka.html-reporter
  junit:
    on: [run]
    resolver: chiperka.junit-reporter
  my-custom:
    on: [run]
    resolver: ./scripts/my-report.sh
` + "```" + `

Reports config: each key is a report type name. ` + "`" + `on` + "`" + ` lists scopes (test, run, global).
` + "`" + `resolver` + "`" + ` is either a built-in (chiperka.*) or a custom shell command.
Custom resolvers receive env vars CHIPERKA_REPORT_SCOPE, CHIPERKA_REPORT_SCOPE_ID,
CHIPERKA_REPORT_OUTPUT_DIR, and result JSON on stdin.

## MCP Server

Start with ` + "`" + `chiperka mcp` + "`" + `. Configure in .mcp.json or claude_desktop_config.json.

### Discovery tools
  chiperka_context                    - This reference document
  chiperka_list(kind)                 - List resources: test, service, or endpoint
  chiperka_get(kind, name)            - Full detail of a single resource

### Execution tools
  chiperka_validate(path)             - Validate without executing
  chiperka_execute(yaml)              - Run inline YAML test (probe endpoints)
  chiperka_run(path)                  - Execute tests, persist results

### Result tools (progressive disclosure)
  chiperka_read_runs()                - List recent runs
  chiperka_read_run(uuid)             - Run summary with test UUIDs
  chiperka_read_test(uuid)            - Test detail with artifact names
  chiperka_read_artifact(uuid, name)  - Raw artifact content

### Report tools
  chiperka_report_types()             - Available report types from config
  chiperka_report_generate(type, scope, scope_id) - Generate a report
  chiperka_report_list(scope?, scope_id?)          - List generated reports
  chiperka_report_get(type, scope, scope_id, file?) - Read report metadata or file

## Recommended AI agent workflow

### 1. Understand the project
  - Call chiperka_context (this document)
  - Call chiperka_list(kind:"endpoint") to see what can be tested
  - Call chiperka_list(kind:"service") to see available services
  - Call chiperka_list(kind:"test") to see existing tests

### 2. Find coverage gaps
  - Compare endpoints vs tests — endpoints without tests need coverage
  - Use chiperka_get(kind:"endpoint", name:"...") to see required inputs

### 3. Write and run tests
  - Write a .chiperka file (kind: test) referencing the endpoint's service
  - Use chiperka_execute(yaml) to probe the endpoint first (see what it returns)
  - Add assertions based on observed behavior
  - Run with chiperka_run(path) to execute and persist results

### 4. Analyze results
  - chiperka_read_run(uuid) to see pass/fail summary
  - chiperka_read_test(uuid) for failed tests — check assertions, HTTP exchanges
  - chiperka_read_artifact(uuid, name) for response bodies and logs
  - Fix tests and re-run until green

### 5. Generate reports
  - chiperka_report_types() to see what's available
  - chiperka_report_generate(type, scope, scope_id) to create reports
  - chiperka_report_get(...) to read generated reports

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All tests passed / validation OK |
| 1 | Test assertion failures / general error |
| 2 | Infrastructure error (service startup, healthcheck, setup failed) |
| 3 | Validation errors (chiperka validate only) |

Run UUID prefixes: lr- (local run), cr- (cloud run).`
