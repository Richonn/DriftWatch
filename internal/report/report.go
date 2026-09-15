package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Richonn/driftwatch/internal/diff"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

var (
	colorOK     = color.New(color.FgGreen, color.Bold)
	colorDrift  = color.New(color.FgRed, color.Bold)
	colorWarn   = color.New(color.FgYellow, color.Bold)
	colorHeader = color.New(color.FgCyan, color.Bold)
	colorMuted  = color.New(color.Faint)
)

func Render(w io.Writer, report *diff.DriftReport, format string) bool {
	switch format {
	case "json":
		renderJSON(w, report)
	case "yaml":
		renderYAML(w, report)
	default:
		renderTable(w, report)
	}
	return report.DriftCount > 0
}

func renderTable(w io.Writer, report *diff.DriftReport) {
	colorHeader.Fprintf(w, "\nDriftWatch Scan Report\n")
	colorMuted.Fprintf(w, "Scanned at : %s\n", report.ScannedAt.Format("2006-01-02 15:04:05"))
	colorMuted.Fprintf(w, "Cluster    : %s\n", report.ClusterContext)
	colorMuted.Fprintf(w, "GitOps repo: %s\n\n", report.GitOpsRepo)

	const (
		wKind   = 16
		wName   = 36
		wNS     = 20
		wStatus = 0
	)

	colorHeader.Fprintf(w, "%-*s %-*s %-*s %s\n", wKind, "KIND", wName, "NAME", wNS, "NAMESPACE", "STATUS")
	fmt.Fprintln(w, strings.Repeat("─", 100))

	if len(report.Drifts) == 0 {
		colorOK.Fprintf(w, "  ✓ No drift detected — all resources are in sync.\n")
	} else {
		for _, item := range report.Drifts {
			status, statusColor := formatStatus(item)
			statusColor.Fprintf(w, "%-*s %-*s %-*s %s\n",
				wKind, item.Kind,
				wName, truncate(item.Name, wName),
				wNS, truncate(item.Namespace, wNS),
				status,
			)
			for _, detail := range item.Details {
				colorMuted.Fprintf(w, "  %s  └─ %s\n", strings.Repeat(" ", wKind+wName+wNS), detail)
			}
		}
	}

	fmt.Fprintln(w, strings.Repeat("─", 100))
	if report.DriftCount == 0 {
		colorOK.Fprintf(w, "✓ %d resources scanned, 0 drifts detected\n\n", report.TotalResources)
	} else {
		colorDrift.Fprintf(w, "✗ %d resources scanned, %d drift(s) detected\n\n", report.TotalResources, report.DriftCount)
	}
}

func formatStatus(item diff.DriftItem) (string, *color.Color) {
	switch item.DriftType {
	case diff.DriftMissingInCluster:
		return "✗ MISSING IN CLUSTER", colorDrift
	case diff.DriftMissingInGitOps:
		return "⚠ MISSING IN GITOPS", colorWarn
	case diff.DriftSpecDrift:
		detail := ""
		if len(item.Details) > 0 {
			detail = " (" + item.Details[0] + ")"
		}
		return "⚠ SPEC DRIFT" + detail, colorWarn
	default:
		return "✓ IN SYNC", colorOK
	}
}

func renderJSON(w io.Writer, report *diff.DriftReport) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}

func renderYAML(w io.Writer, report *diff.DriftReport) {
	_ = yaml.NewEncoder(w).Encode(report)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func ExitCode(hasDrift, failOnDrift bool) int {
	if hasDrift && failOnDrift {
		return 1
	}
	return 0
}

func RenderToStdout(report *diff.DriftReport, format string, failOnDrift bool) {
	hasDrift := Render(os.Stdout, report, format)
	os.Exit(ExitCode(hasDrift, failOnDrift))
}
