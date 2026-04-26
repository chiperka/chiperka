// Package parser handles reading and parsing of .chiperka files.
//
// Every .chiperka file declares one or more resources using the Kubernetes-style
// shape:
//
//   kind: Service | Endpoint | Test
//   metadata:
//     name: <unique-name>
//   spec:
//     <kind-specific fields>
//
// Multiple documents in a single file are supported (separated by `---`),
// matching Kubernetes manifest conventions:
//
//   kind: Service
//   metadata: {name: postgres}
//   spec: {image: postgres:16}
//   ---
//   kind: Endpoint
//   metadata: {name: user-login}
//   spec: {service: api, endpoint: {method: POST, url: /auth/login}}
//
// The top-level `kind:` field is required on every document and case-sensitive.
// ParseAll walks a list of file paths, dispatches each document by kind, and
// returns a ParseResult with all collections populated.
package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"

	"chiperka-cli/internal/model"
	"gopkg.in/yaml.v3"
)

// envVarPattern matches environment variables with $CHIPERKA_ prefix.
var envVarPattern = regexp.MustCompile(`\$CHIPERKA_[A-Za-z0-9_]+`)

// expandEnvVars replaces all $CHIPERKA_* patterns with their environment variable values.
func expandEnvVars(data []byte) []byte {
	return envVarPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := string(match[1:]) // Remove the $ prefix
		return []byte(os.Getenv(varName))
	})
}

// kindPeek is a minimal struct used to read just the top-level `kind` field
// before deciding how to fully decode a document.
type kindPeek struct {
	Kind string `yaml:"kind"`
}

// detectKindOfNode reads only the top-level `kind:` field from a yaml.Node.
// A missing kind is an error — every document must declare its kind.
func detectKindOfNode(node *yaml.Node) (string, error) {
	var peek kindPeek
	if err := node.Decode(&peek); err != nil {
		return "", err
	}
	if peek.Kind == "" {
		return "", fmt.Errorf("missing required top-level 'kind' field (expected %q, %q, or %q)",
			model.KindService, model.KindEndpoint, model.KindTest)
	}
	return peek.Kind, nil
}

// decodeDocuments splits raw YAML bytes into one yaml.Node per document.
// Empty documents (just whitespace or a stray `---`) are skipped.
func decodeDocuments(data []byte) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var nodes []*yaml.Node
	for {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if node.Kind == 0 {
			continue
		}
		nodeCopy := node
		nodes = append(nodes, &nodeCopy)
	}
	return nodes, nil
}

// Parser reads and parses .chiperka files into model resources.
type Parser struct{}

// New creates a new Parser instance.
func New() *Parser {
	return &Parser{}
}

// ParseResult contains all resources discovered from a set of .chiperka files,
// split by kind.
type ParseResult struct {
	Tests     *model.TestCollection
	Services  *model.ServiceTemplateCollection
	Endpoints *model.EndpointCollection
	Errors    []error
}

// ParseFile reads a single .chiperka file as a Test. The file may contain
// multiple YAML documents, but exactly one must be a Test — otherwise an error
// is returned. Auxiliary documents (Service/Endpoint) in the same file are
// ignored here; load them via ParseAll if you need them.
func (p *Parser) ParseFile(filePath string) (*model.Suite, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return p.parseTestFromBytes(data, filePath)
}

// ParseBytes parses YAML test definition from raw bytes as a Test.
// Used for API/MCP-submitted inline tests. Errors if no Test document is
// present or if more than one Test document is present.
func (p *Parser) ParseBytes(data []byte) (*model.Suite, error) {
	return p.parseTestFromBytes(data, "<api>")
}

func (p *Parser) parseTestFromBytes(data []byte, source string) (*model.Suite, error) {
	data = expandEnvVars(data)

	nodes, err := decodeDocuments(data)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to parse YAML: %w", source, err)
	}

	var test *model.Test
	for i, node := range nodes {
		kind, err := detectKindOfNode(node)
		if err != nil {
			return nil, fmt.Errorf("%s document %d: %w", source, i+1, err)
		}
		if kind != model.KindTest {
			continue // ignore Service/Endpoint docs in this entry point
		}
		if test != nil {
			return nil, fmt.Errorf("%s: contains more than one Test document; ParseFile/ParseBytes expects exactly one", source)
		}
		var t model.Test
		if err := node.Decode(&t); err != nil {
			return nil, fmt.Errorf("%s: failed to decode Test: %w", source, err)
		}
		t.FilePath = source
		test = &t
	}

	if test == nil {
		return nil, fmt.Errorf("%s: no kind: Test document found", source)
	}

	suite := &model.Suite{
		Name:     test.Name,
		Tests:    []model.Test{*test},
		FilePath: source,
	}
	return suite, nil
}

// ParseAll reads multiple .chiperka files, dispatches each document by kind,
// and returns a populated ParseResult. Each file may contain multiple YAML
// documents separated by `---`. Per-document errors are collected in
// result.Errors and do not abort processing of remaining documents/files.
func (p *Parser) ParseAll(filePaths []string) *ParseResult {
	result := &ParseResult{
		Tests:     model.NewTestCollection(),
		Services:  model.NewServiceTemplateCollection(),
		Endpoints: model.NewEndpointCollection(),
		Errors:    make([]error, 0),
	}

	for _, path := range filePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to read file %s: %w", path, err))
			continue
		}

		data = expandEnvVars(data)

		nodes, err := decodeDocuments(data)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to parse YAML in %s: %w", path, err))
			continue
		}

		for i, node := range nodes {
			docID := docLocation(path, len(nodes), i)
			p.dispatchDocument(node, path, docID, result)
		}
	}

	return result
}

// docLocation returns a human-readable location for a document in a file.
// Single-doc files just use the path; multi-doc files include "doc N".
func docLocation(path string, total, idx int) string {
	if total <= 1 {
		return path
	}
	return fmt.Sprintf("%s (doc %d)", path, idx+1)
}

// dispatchDocument decodes one yaml.Node into the right resource type and
// adds it to the result. Errors are appended to result.Errors and do not
// abort the caller.
func (p *Parser) dispatchDocument(node *yaml.Node, filePath, docID string, result *ParseResult) {
	kind, err := detectKindOfNode(node)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("%s: %w", docID, err))
		return
	}

	switch kind {
	case model.KindTest:
		var test model.Test
		if err := node.Decode(&test); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: failed to decode Test: %w", docID, err))
			return
		}
		if test.Name == "" {
			result.Errors = append(result.Errors, fmt.Errorf("%s: test is missing required 'metadata.name' field", docID))
			return
		}
		test.FilePath = filePath
		result.Tests.AddTest(test)

	case model.KindService:
		var template model.ServiceTemplate
		if err := node.Decode(&template); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: failed to decode Service: %w", docID, err))
			return
		}
		if template.Name == "" {
			result.Errors = append(result.Errors, fmt.Errorf("%s: service is missing required 'metadata.name' field", docID))
			return
		}
		if existing := result.Services.GetTemplate(template.Name); existing != nil {
			result.Errors = append(result.Errors, fmt.Errorf(
				"%s: duplicate service name %q (already declared in %s)",
				docID, template.Name, existing.FilePath,
			))
			return
		}
		template.FilePath = filePath
		result.Services.AddTemplate(&template)

	case model.KindEndpoint:
		var ep model.Endpoint
		if err := node.Decode(&ep); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: failed to decode Endpoint: %w", docID, err))
			return
		}
		if ep.Name == "" {
			result.Errors = append(result.Errors, fmt.Errorf("%s: endpoint is missing required 'metadata.name' field", docID))
			return
		}
		if existing := result.Endpoints.GetEndpoint(ep.Name); existing != nil {
			result.Errors = append(result.Errors, fmt.Errorf(
				"%s: duplicate endpoint name %q (already declared in %s)",
				docID, ep.Name, existing.FilePath,
			))
			return
		}
		ep.FilePath = filePath
		result.Endpoints.AddEndpoint(&ep)

	default:
		result.Errors = append(result.Errors, fmt.Errorf("%s: unknown kind %q (expected %q, %q, or %q)",
			docID, kind, model.KindTest, model.KindService, model.KindEndpoint))
	}
}
