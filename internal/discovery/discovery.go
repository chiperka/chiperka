// Package discovery provides shared logic for discovering and querying
// .chiperka resources (tests, services, endpoints) from the filesystem.
// Both the CLI commands and the MCP server use this package.
package discovery

import (
	"fmt"

	"chiperka-cli/internal/config"
	"chiperka-cli/internal/finder"
	"chiperka-cli/internal/model"
	"chiperka-cli/internal/parser"
)

// Result holds all parsed resources from a discovery scan.
type Result struct {
	Tests     *model.TestCollection
	Services  *model.ServiceTemplateCollection
	Endpoints *model.EndpointCollection
	Errors    []error
}

// All discovers and parses all .chiperka files using the configured discovery
// paths (from chiperka.yaml) or falling back to the current directory.
func All() (*Result, error) {
	return AllWithConfig("")
}

// AllWithConfig discovers and parses all .chiperka files using the given
// config file path. If configFile is empty, auto-discovers configuration.
func AllWithConfig(configFile string) (*Result, error) {
	cfg, _ := loadConfig(configFile)
	var paths []string
	if cfg != nil && len(cfg.Discovery) > 0 {
		paths = cfg.Discovery
	} else {
		paths = []string{"."}
	}

	files, err := finder.FindAll(paths)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return &Result{
			Tests:     model.NewTestCollection(),
			Services:  model.NewServiceTemplateCollection(),
			Endpoints: model.NewEndpointCollection(),
		}, nil
	}

	p := parser.New()
	parsed := p.ParseAll(files)

	return &Result{
		Tests:     parsed.Tests,
		Services:  parsed.Services,
		Endpoints: parsed.Endpoints,
		Errors:    parsed.Errors,
	}, nil
}

// ListTest is a compact test summary returned by ListTests.
type ListTest struct {
	Name  string   `json:"name"`
	Suite string   `json:"suite"`
	Tags  []string `json:"tags,omitempty"`
	File  string   `json:"file"`
}

// ListTests returns a compact list of all tests.
func ListTests(r *Result) []ListTest {
	var items []ListTest
	for _, suite := range r.Tests.Suites {
		for _, test := range suite.Tests {
			items = append(items, ListTest{
				Name:  test.Name,
				Suite: suite.Name,
				Tags:  test.Tags,
				File:  suite.FilePath,
			})
		}
	}
	if items == nil {
		items = []ListTest{}
	}
	return items
}

// ListService is a compact service summary returned by ListServices.
type ListService struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	File  string `json:"file"`
}

// ListServices returns a compact list of all service templates.
func ListServices(r *Result) []ListService {
	var items []ListService
	for _, tmpl := range r.Services.Templates {
		items = append(items, ListService{
			Name:  tmpl.Name,
			Image: tmpl.Image,
			File:  tmpl.FilePath,
		})
	}
	if items == nil {
		items = []ListService{}
	}
	return items
}

// ListEndpoint is a compact endpoint summary returned by ListEndpoints.
type ListEndpoint struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Method  string `json:"method"`
	URL     string `json:"url"`
	File    string `json:"file"`
}

// ListEndpoints returns a compact list of all endpoints.
func ListEndpoints(r *Result) []ListEndpoint {
	var items []ListEndpoint
	for _, ep := range r.Endpoints.Endpoints {
		method, url := "", ""
		if ep.HTTP != nil {
			method = ep.HTTP.Method
			url = ep.HTTP.URL
		} else if ep.Command != nil {
			method = "CLI"
			url = ep.Command.Cmd
		}
		items = append(items, ListEndpoint{
			Name:    ep.Name,
			Service: ep.Service,
			Method:  method,
			URL:     url,
			File:    ep.FilePath,
		})
	}
	if items == nil {
		items = []ListEndpoint{}
	}
	return items
}

// TestDetail contains full detail for a single test.
type TestDetail struct {
	Name       string                `json:"name"`
	Suite      string                `json:"suite"`
	File       string                `json:"file"`
	Tags       []string              `json:"tags,omitempty"`
	Skipped    bool                  `json:"skipped,omitempty"`
	Services   []model.Service       `json:"services,omitempty"`
	Setup      []model.SetupInstruction `json:"setup,omitempty"`
	Execution  model.Execution       `json:"execution"`
	Assertions []model.Assertion     `json:"assertions,omitempty"`
	Teardown   []model.SetupInstruction `json:"teardown,omitempty"`
}

// GetTest finds a test by name and returns its full detail.
func GetTest(r *Result, name string) (*TestDetail, error) {
	for _, suite := range r.Tests.Suites {
		for _, test := range suite.Tests {
			if test.Name == name {
				return &TestDetail{
					Name:       test.Name,
					Suite:      suite.Name,
					File:       suite.FilePath,
					Tags:       test.Tags,
					Skipped:    test.Skipped,
					Services:   test.Services,
					Setup:      test.Setup,
					Execution:  test.Execution,
					Assertions: test.Assertions,
					Teardown:   test.Teardown,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("test %q not found", name)
}

// GetService finds a service template by name and returns it.
func GetService(r *Result, name string) (*model.ServiceTemplate, error) {
	tmpl := r.Services.GetTemplate(name)
	if tmpl == nil {
		return nil, fmt.Errorf("service %q not found", name)
	}
	return tmpl, nil
}

// GetEndpoint finds an endpoint by name and returns it.
func GetEndpoint(r *Result, name string) (*model.Endpoint, error) {
	ep := r.Endpoints.GetEndpoint(name)
	if ep == nil {
		return nil, fmt.Errorf("endpoint %q not found", name)
	}
	return ep, nil
}

func loadConfig(configFile string) (*config.Config, error) {
	if configFile != "" {
		return config.Load(configFile)
	}
	cfg, _ := config.Discover()
	return cfg, nil
}
