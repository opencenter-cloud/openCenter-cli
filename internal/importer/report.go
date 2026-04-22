package importer

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func RenderScanResult(result *ImportScanResult, format string) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("scan result cannot be nil")
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return json.MarshalIndent(result, "", "  ")
	case "yaml":
		return yaml.Marshal(result)
	case "", "text":
		var b strings.Builder
		fmt.Fprintf(&b, "Repo: %s\n", result.RepoPath)
		fmt.Fprintf(&b, "Clusters discovered: %d\n", result.Summary.ClustersDiscovered)
		fmt.Fprintf(&b, "Conflicts: %d\n", result.Summary.ConflictCount)
		fmt.Fprintf(&b, "Skipped fields: %d\n", result.Summary.SkippedFieldCount)
		if len(result.Clusters) > 0 {
			fmt.Fprintf(&b, "\nClusters:\n")
			for _, cluster := range result.Clusters {
				fmt.Fprintf(&b, "  - %s", cluster.ClusterName)
				if cluster.Organization != "" {
					fmt.Fprintf(&b, " (%s)", cluster.Organization)
				}
				fmt.Fprintf(&b, "\n")
				if len(cluster.Conflicts) > 0 {
					fmt.Fprintf(&b, "    conflicts: %d\n", len(cluster.Conflicts))
				}
				if len(cluster.SkippedFields) > 0 {
					fmt.Fprintf(&b, "    skipped: %d\n", len(cluster.SkippedFields))
				}
			}
		}
		return []byte(b.String()), nil
	default:
		return nil, fmt.Errorf("unsupported report output format %q", format)
	}
}
