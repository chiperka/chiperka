package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"chiperka-cli/internal/report"
)

func reportTypesTool() mcp.Tool {
	return mcp.NewTool("chiperka_report_types",
		mcp.WithDescription("List available report types from configuration. Shows what reports can be generated and for which scopes (run, test, global)."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
}

func reportGenerateTool() mcp.Tool {
	return mcp.NewTool("chiperka_report_generate",
		mcp.WithDescription("Generate a report. Requires a type (from chiperka_report_types) and scope (run/test/global with optional UUID)."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("type",
			mcp.Description("Report type (e.g. html, junit)"),
			mcp.Required(),
		),
		mcp.WithString("scope",
			mcp.Description("Report scope: run, test, or global"),
			mcp.Required(),
		),
		mcp.WithString("scope_id",
			mcp.Description("Run or test UUID (required for run/test scope)"),
		),
	)
}

func reportListTool() mcp.Tool {
	return mcp.NewTool("chiperka_report_list",
		mcp.WithDescription("List generated reports. Optionally filter by scope and scope_id."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("scope",
			mcp.Description("Filter by scope: run, test, or global"),
		),
		mcp.WithString("scope_id",
			mcp.Description("Filter by scope UUID"),
		),
	)
}

func reportGetTool() mcp.Tool {
	return mcp.NewTool("chiperka_report_get",
		mcp.WithDescription("Read a generated report's metadata and file list. Use file parameter to read a specific file from the report."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("type",
			mcp.Description("Report type (e.g. html, junit)"),
			mcp.Required(),
		),
		mcp.WithString("scope",
			mcp.Description("Report scope: run, test, or global"),
			mcp.Required(),
		),
		mcp.WithString("scope_id",
			mcp.Description("Run or test UUID"),
		),
		mcp.WithString("file",
			mcp.Description("Read a specific file from the report (returns raw content)"),
		),
	)
}

func handleReportTypes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := loadConfig("")
	if err != nil {
		return nil, err
	}
	return jsonResult(report.AvailableTypes(cfg))
}

func handleReportGenerate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reportType, _ := request.GetArguments()["type"].(string)
	if reportType == "" {
		return nil, fmt.Errorf("type is required")
	}
	scope, _ := request.GetArguments()["scope"].(string)
	if scope == "" {
		return nil, fmt.Errorf("scope is required")
	}
	scopeID, _ := request.GetArguments()["scope_id"].(string)

	cfg, err := loadConfig("")
	if err != nil {
		return nil, err
	}

	meta, err := report.Generate(cfg, reportType, scope, scopeID)
	if err != nil {
		return nil, err
	}

	return jsonResult(meta)
}

func handleReportList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	scope, _ := request.GetArguments()["scope"].(string)
	scopeID, _ := request.GetArguments()["scope_id"].(string)

	refs, err := report.List(scope, scopeID)
	if err != nil {
		return nil, err
	}

	return jsonResult(refs)
}

func handleReportGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reportType, _ := request.GetArguments()["type"].(string)
	if reportType == "" {
		return nil, fmt.Errorf("type is required")
	}
	scope, _ := request.GetArguments()["scope"].(string)
	if scope == "" {
		return nil, fmt.Errorf("scope is required")
	}
	scopeID, _ := request.GetArguments()["scope_id"].(string)
	fileName, _ := request.GetArguments()["file"].(string)

	if fileName != "" {
		content, err := report.GetFile(reportType, scope, scopeID, fileName)
		if err != nil {
			return nil, err
		}
		// Truncate large files
		const maxSize = 32 * 1024
		text := string(content)
		truncated := false
		if len(text) > maxSize {
			text = text[:maxSize]
			truncated = true
		}
		resp := map[string]interface{}{
			"file":    fileName,
			"content": text,
			"size":    len(content),
		}
		if truncated {
			resp["truncated"] = true
		}
		return jsonResult(resp)
	}

	meta, err := report.Get(reportType, scope, scopeID)
	if err != nil {
		return nil, err
	}

	return jsonResult(meta)
}
