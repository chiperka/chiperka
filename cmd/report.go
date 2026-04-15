package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"chiperka-cli/internal/config"
	"chiperka-cli/internal/report"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate, list, and read reports",
	Long:  "Manage reports generated from test results or specification data.",
}

// --- report generate ---

var reportGenerateCmd = &cobra.Command{
	Use:   "generate <type>",
	Short: "Generate a report",
	Args:  cobra.ExactArgs(1),
	RunE:  runReportGenerate,
}

var (
	reportRunUUID  string
	reportTestUUID string
	reportJSONFlag bool
)

// --- report list ---

var reportListCmd = &cobra.Command{
	Use:   "list",
	Short: "List generated reports",
	RunE:  runReportList,
}

var (
	reportListScope   string
	reportListScopeID string
)

// --- report get ---

var reportGetCmd = &cobra.Command{
	Use:   "get <type>",
	Short: "Read a generated report",
	Args:  cobra.ExactArgs(1),
	RunE:  runReportGet,
}

var (
	reportGetRunUUID  string
	reportGetTestUUID string
	reportGetFile     string
)

// --- report types ---

var reportTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "Show available report types from configuration",
	RunE:  runReportTypes,
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.AddCommand(reportGenerateCmd, reportListCmd, reportGetCmd, reportTypesCmd)

	// generate flags
	reportGenerateCmd.Flags().StringVar(&reportRunUUID, "run", "", "Run UUID")
	reportGenerateCmd.Flags().StringVar(&reportTestUUID, "test", "", "Test UUID")
	reportGenerateCmd.Flags().BoolVar(&reportJSONFlag, "json", false, "Output as JSON")

	// list flags
	reportListCmd.Flags().StringVar(&reportListScope, "scope", "", "Filter by scope (run, test, global)")
	reportListCmd.Flags().StringVar(&reportListScopeID, "scope-id", "", "Filter by scope UUID")
	reportListCmd.Flags().BoolVar(&reportJSONFlag, "json", false, "Output as JSON")

	// get flags
	reportGetCmd.Flags().StringVar(&reportGetRunUUID, "run", "", "Run UUID")
	reportGetCmd.Flags().StringVar(&reportGetTestUUID, "test", "", "Test UUID")
	reportGetCmd.Flags().StringVar(&reportGetFile, "file", "", "Read a specific file from the report")
	reportGetCmd.Flags().BoolVar(&reportJSONFlag, "json", false, "Output as JSON")

	// types flags
	reportTypesCmd.Flags().BoolVar(&reportJSONFlag, "json", false, "Output as JSON")
}

func resolveScope(runUUID, testUUID string) (string, string, error) {
	if runUUID != "" && testUUID != "" {
		return "", "", fmt.Errorf("specify either --run or --test, not both")
	}
	if runUUID != "" {
		return report.ScopeRun, runUUID, nil
	}
	if testUUID != "" {
		return report.ScopeTest, testUUID, nil
	}
	return report.ScopeGlobal, "", nil
}

func loadReportConfig() (*config.Config, error) {
	if configFile != "" {
		return config.Load(configFile)
	}
	cfg, found := config.Discover()
	if !found {
		return cfg, nil
	}
	return cfg, nil
}

func runReportGenerate(cmd *cobra.Command, args []string) error {
	reportType := args[0]

	scope, scopeID, err := resolveScope(reportRunUUID, reportTestUUID)
	if err != nil {
		return exitErrorf(ExitValidationError, "%v", err)
	}

	cfg, err := loadReportConfig()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	meta, err := report.Generate(cfg, reportType, scope, scopeID)
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	if reportJSONFlag {
		return outputJSON(meta)
	}

	fmt.Printf("Report %q generated for %s", meta.Type, meta.Scope)
	if meta.ScopeID != "" {
		fmt.Printf(" %s", meta.ScopeID)
	}
	fmt.Printf("\n")
	fmt.Printf("Files: %d\n", len(meta.Files))
	for _, f := range meta.Files {
		fmt.Printf("  %s\n", f)
	}
	return nil
}

func runReportList(cmd *cobra.Command, args []string) error {
	refs, err := report.List(reportListScope, reportListScopeID)
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	if reportJSONFlag {
		return outputJSON(refs)
	}

	if len(refs) == 0 {
		fmt.Println("No reports found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tSCOPE\tSCOPE_ID\tGENERATED_AT\tPATH")
	for _, r := range refs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.Type, r.Scope, r.ScopeID,
			r.GeneratedAt.Format("2006-01-02 15:04:05"),
			r.Path)
	}
	w.Flush()
	return nil
}

func runReportGet(cmd *cobra.Command, args []string) error {
	reportType := args[0]

	scope, scopeID, err := resolveScope(reportGetRunUUID, reportGetTestUUID)
	if err != nil {
		return exitErrorf(ExitValidationError, "%v", err)
	}

	// If --file specified, read and output that file
	if reportGetFile != "" {
		content, err := report.GetFile(reportType, scope, scopeID, reportGetFile)
		if err != nil {
			return exitErrorf(ExitInfraError, "%v", err)
		}
		os.Stdout.Write(content)
		return nil
	}

	// Otherwise return metadata
	meta, err := report.Get(reportType, scope, scopeID)
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	if reportJSONFlag {
		return outputJSON(meta)
	}

	fmt.Printf("Type: %s\n", meta.Type)
	fmt.Printf("Scope: %s\n", meta.Scope)
	if meta.ScopeID != "" {
		fmt.Printf("ScopeID: %s\n", meta.ScopeID)
	}
	fmt.Printf("Generated: %s\n", meta.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Resolver: %s\n", meta.Resolver)
	if len(meta.Files) > 0 {
		fmt.Println("\nFiles:")
		for _, f := range meta.Files {
			fmt.Printf("  %s\n", f)
		}
	}
	return nil
}

func runReportTypes(cmd *cobra.Command, args []string) error {
	cfg, err := loadReportConfig()
	if err != nil {
		return exitErrorf(ExitInfraError, "%v", err)
	}

	types := report.AvailableTypes(cfg)

	if reportJSONFlag {
		return outputJSON(types)
	}

	if len(types) == 0 {
		fmt.Println("No report types configured. Add reports section to chiperka.yaml.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tSCOPES\tRESOLVER")
	for _, t := range types {
		fmt.Fprintf(w, "%s\t%v\t%s\n", t.Type, t.Scopes, t.Resolver)
	}
	w.Flush()
	return nil
}
