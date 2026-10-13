package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Richonn/driftwatch/internal/diff"
)

type payload struct {
	Text string `json:"text"`
}

func Send(webhookURL string, report *diff.DriftReport) error {
	msg := buildMessage(report)
	body, err := json.Marshal(payload{Text: msg})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

func buildMessage(report *diff.DriftReport) string {
	if report.DriftCount == 0 {
		return fmt.Sprintf(":white_check_mark: *DriftWatch* — no drift detected (%d resources scanned)\nRepo: %s",
			report.TotalResources, report.GitOpsRepo)
	}

	msg := fmt.Sprintf(":warning: *DriftWatch* — %d drift(s) detected out of %d resources\nRepo: %s\n",
		report.DriftCount, report.TotalResources, report.GitOpsRepo)

	for _, item := range report.Drifts {
		icon := "⚠"
		if item.DriftType == diff.DriftMissingInCluster {
			icon = "✗"
		}
		msg += fmt.Sprintf("%s `%s/%s` (%s): %s\n", icon, item.Namespace, item.Name, item.Kind, item.DriftType)
		for _, d := range item.Details {
			msg += fmt.Sprintf("  • %s\n", d)
		}
	}

	return msg
}
