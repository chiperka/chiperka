package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"chiperka-cli/internal/discovery"
)

var getCmd = &cobra.Command{
	Use:   "get <kind> <name>",
	Short: "Show detail of a resource by kind and name",
	Long:  "Show full detail of a single resource (test, service, endpoint) by name.",
}

var getTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Show test detail",
	Args:  cobra.ExactArgs(1),
	RunE:  runGetTest,
}

var getServiceCmd = &cobra.Command{
	Use:   "service <name>",
	Short: "Show service template detail",
	Args:  cobra.ExactArgs(1),
	RunE:  runGetService,
}

var getEndpointCmd = &cobra.Command{
	Use:   "endpoint <name>",
	Short: "Show endpoint detail",
	Args:  cobra.ExactArgs(1),
	RunE:  runGetEndpoint,
}

var getJSONFlag bool

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getTestCmd, getServiceCmd, getEndpointCmd)

	getCmd.PersistentFlags().BoolVar(&getJSONFlag, "json", false, "Output as JSON")
}

func runGetTest(cmd *cobra.Command, args []string) error {
	result, err := discovery.All()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	detail, err := discovery.GetTest(result, args[0])
	if err != nil {
		return exitErrorf(ExitValidationError, "%v", err)
	}

	if getJSONFlag {
		return outputJSON(detail)
	}

	fmt.Printf("Name: %s\n", detail.Name)
	fmt.Printf("Suite: %s\n", detail.Suite)
	fmt.Printf("File: %s\n", detail.File)
	if len(detail.Tags) > 0 {
		fmt.Printf("Tags: %v\n", detail.Tags)
	}
	if detail.Skipped {
		fmt.Println("Skipped: true")
	}
	if len(detail.Services) > 0 {
		fmt.Println("\nServices:")
		for _, svc := range detail.Services {
			if svc.Ref != "" {
				fmt.Printf("  - %s (ref: %s)\n", svc.Name, svc.Ref)
			} else {
				fmt.Printf("  - %s (%s)\n", svc.Name, svc.Image)
			}
		}
	}
	exec := detail.Execution
	executor := string(exec.Executor)
	if executor == "" {
		executor = "http"
	}
	fmt.Printf("\nExecution: %s\n", executor)
	if executor == "http" {
		fmt.Printf("  Target: %s\n", exec.Target)
		fmt.Printf("  %s %s\n", exec.Request.Method, exec.Request.URL)
	} else if exec.CLI != nil {
		fmt.Printf("  Service: %s\n", exec.CLI.Service)
		fmt.Printf("  Command: %s\n", exec.CLI.Command)
	}
	if len(detail.Assertions) > 0 {
		fmt.Printf("\nAssertions: %d\n", len(detail.Assertions))
	}
	return nil
}

func runGetService(cmd *cobra.Command, args []string) error {
	result, err := discovery.All()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	tmpl, err := discovery.GetService(result, args[0])
	if err != nil {
		return exitErrorf(ExitValidationError, "%v", err)
	}

	if getJSONFlag {
		return outputJSON(tmpl)
	}

	fmt.Printf("Name: %s\n", tmpl.Name)
	fmt.Printf("Image: %s\n", tmpl.Image)
	fmt.Printf("File: %s\n", tmpl.FilePath)
	if len(tmpl.Command) > 0 {
		fmt.Printf("Command: %v\n", []string(tmpl.Command))
	}
	if tmpl.WorkingDir != "" {
		fmt.Printf("WorkingDir: %s\n", tmpl.WorkingDir)
	}
	if len(tmpl.Environment) > 0 {
		fmt.Println("\nEnvironment:")
		for k, v := range tmpl.Environment {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}
	if tmpl.HealthCheck != nil && tmpl.HealthCheck.Test != "" {
		fmt.Printf("\nHealthcheck: %s\n", tmpl.HealthCheck.Test)
	}
	if tmpl.Weight > 0 {
		fmt.Printf("Weight: %d\n", tmpl.Weight)
	}
	if len(tmpl.Artifacts) > 0 {
		fmt.Println("\nArtifacts:")
		for _, a := range tmpl.Artifacts {
			fmt.Printf("  - %s (%s)\n", a.Name, a.Path)
		}
	}
	if len(tmpl.Hooks) > 0 {
		fmt.Printf("\nHooks: %d\n", len(tmpl.Hooks))
	}
	return nil
}

func runGetEndpoint(cmd *cobra.Command, args []string) error {
	result, err := discovery.All()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	ep, err := discovery.GetEndpoint(result, args[0])
	if err != nil {
		return exitErrorf(ExitValidationError, "%v", err)
	}

	if getJSONFlag {
		return outputJSON(ep)
	}

	fmt.Printf("Name: %s\n", ep.Name)
	fmt.Printf("Service: %s\n", ep.Service)
	if ep.HTTP != nil {
		fmt.Printf("Method: %s\n", ep.HTTP.Method)
		fmt.Printf("URL: %s\n", ep.HTTP.URL)
	}
	if ep.Command != nil {
		fmt.Printf("Command: %s\n", ep.Command.Cmd)
	}
	fmt.Printf("File: %s\n", ep.FilePath)
	if ep.Command != nil && len(ep.Command.Args) > 0 {
		fmt.Println("\nArguments:")
		for _, arg := range ep.Command.Args {
			fmt.Printf("  - %s", arg.Name)
			if arg.Default != "" {
				fmt.Printf(" (default: %s)", arg.Default)
			}
			if arg.Description != "" {
				fmt.Printf(" — %s", arg.Description)
			}
			fmt.Println()
		}
	}
	return nil
}
