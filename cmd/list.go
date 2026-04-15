package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"chiperka-cli/internal/discovery"
)

var listCmd = &cobra.Command{
	Use:   "list <kind>",
	Short: "List resources by kind",
	Long:  "List all resources of a given kind (test, service, endpoint) discovered from .chiperka files.",
}

var listTestCmd = &cobra.Command{
	Use:   "test",
	Short: "List all tests",
	RunE:  runListTest,
}

var listServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "List all service templates",
	RunE:  runListService,
}

var listEndpointCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "List all endpoints",
	RunE:  runListEndpoint,
}

var listJSONFlag bool

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(listTestCmd, listServiceCmd, listEndpointCmd)

	listCmd.PersistentFlags().BoolVar(&listJSONFlag, "json", false, "Output as JSON")
}

func runListTest(cmd *cobra.Command, args []string) error {
	result, err := discovery.All()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	items := discovery.ListTests(result)

	if listJSONFlag {
		return outputJSON(items)
	}

	if len(items) == 0 {
		fmt.Println("No tests found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSUITE\tTAGS\tFILE")
	for _, t := range items {
		tags := ""
		if len(t.Tags) > 0 {
			tags = fmt.Sprintf("%v", t.Tags)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Name, t.Suite, tags, t.File)
	}
	w.Flush()
	return nil
}

func runListService(cmd *cobra.Command, args []string) error {
	result, err := discovery.All()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	items := discovery.ListServices(result)

	if listJSONFlag {
		return outputJSON(items)
	}

	if len(items) == 0 {
		fmt.Println("No services found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tIMAGE\tFILE")
	for _, s := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Image, s.File)
	}
	w.Flush()
	return nil
}

func runListEndpoint(cmd *cobra.Command, args []string) error {
	result, err := discovery.All()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	items := discovery.ListEndpoints(result)

	if listJSONFlag {
		return outputJSON(items)
	}

	if len(items) == 0 {
		fmt.Println("No endpoints found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSERVICE\tMETHOD\tURL\tFILE")
	for _, ep := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ep.Name, ep.Service, ep.Method, ep.URL, ep.File)
	}
	w.Flush()
	return nil
}
