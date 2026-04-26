package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chiperka-cli/internal/telemetry"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Chiperka project",
	Long: `Init scaffolds a starter Chiperka project in the current directory using
the recommended layout from the spec documentation:

  .chiperka/
    chiperka.yaml                                    Discovery config
    spec/
      services/api.chiperka                          kind: Service
      endpoints/api/api-root.chiperka                kind: Endpoint
      tests/api/api-root/responds-with-200.chiperka  kind: Test

If .chiperka/chiperka.yaml already exists, init exits without modifying anything.

Example:
  mkdir my-project && cd my-project
  chiperka init
  chiperka test`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const chiperkaYAMLContent = `discovery:
  - ./.chiperka/spec
`

const apiServiceContent = `kind: Service
metadata:
  name: api
  description: "Example API service"
spec:
  image: nginx:alpine
  healthcheck:
    test: "wget -q --spider http://localhost:80/"
    retries: 30
`

const apiRootEndpointContent = `kind: Endpoint
metadata:
  name: api-root
  description: "Default homepage"
  tags: [public]
spec:
  service: api
  endpoint:
    method: GET
    url: /
    response:
      statusCode: 200
`

const apiRootTestRespondsContent = `kind: Test
metadata:
  name: responds-with-200
  description: "GET / returns 200"
  tags: [smoke]
spec:
  endpoint: api-root
  services:
    - ref: api
  execution:
    target: http://api
    request:
      method: GET
      url: /
  assertions:
    - response:
        statusCode: 200
`

const apiRootTestNotFoundContent = `kind: Test
metadata:
  name: missing-page-returns-404
  description: "Unknown URLs return 404"
  tags: [smoke]
spec:
  endpoint: api-root
  services:
    - ref: api
  execution:
    target: http://api
    request:
      method: GET
      url: /missing-page
  assertions:
    - response:
        statusCode: 404
`

// scaffoldFile is one file the init command creates, with its full path under
// the project root and the content to write.
type scaffoldFile struct {
	path    string
	content string
}

var scaffoldFiles = []scaffoldFile{
	{".chiperka/chiperka.yaml", chiperkaYAMLContent},
	{".chiperka/spec/services/api.chiperka", apiServiceContent},
	{".chiperka/spec/endpoints/api/api-root.chiperka", apiRootEndpointContent},
	{".chiperka/spec/tests/api/api-root/responds-with-200.chiperka", apiRootTestRespondsContent},
	{".chiperka/spec/tests/api/api-root/missing-page-returns-404.chiperka", apiRootTestNotFoundContent},
}

func runInit(cmd *cobra.Command, args []string) error {
	telemetry.ShowNoticeIfNeeded(false)
	startTime := time.Now()
	defer func() {
		telemetry.RecordCommand(Version, "init", "", true, time.Since(startTime).Milliseconds())
		telemetry.Wait(2 * time.Second)
	}()

	configPath := filepath.Join(".chiperka", "chiperka.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Println(".chiperka/chiperka.yaml already exists, skipping initialization")
		return nil
	}

	// Create files (with directories on the fly)
	for _, f := range scaffoldFiles {
		if err := os.MkdirAll(filepath.Dir(f.path), 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(f.path), err)
		}
		if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", f.path, err)
		}
	}

	// Add .chiperka/results/ to .gitignore if it exists
	if _, err := os.Stat(".gitignore"); err == nil {
		content, err := os.ReadFile(".gitignore")
		if err == nil && !containsLine(string(content), ".chiperka/results/") {
			f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				if len(content) > 0 && content[len(content)-1] != '\n' {
					f.Write([]byte("\n"))
				}
				f.Write([]byte(".chiperka/results/\n"))
				f.Close()
				fmt.Println("Added .chiperka/results/ to .gitignore")
			}
		}
	}

	for _, f := range scaffoldFiles {
		fmt.Printf("Created %s\n", f.path)
	}
	fmt.Println()
	fmt.Println("Run your tests with: chiperka test")

	return nil
}

func containsLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}
