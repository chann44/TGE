package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OSVPackageQuery struct {
	Name      string
	Ecosystem string
	Version   string
}

type OSVAdvisory struct {
	ID           string
	Aliases      []string
	Summary      string
	Details      string
	Severity     string
	FixedVersion string
	ReferenceURL string
}

const osvQueryBatchURL = "https://api.osv.dev/v1/querybatch"
const osvBatchChunkSize = 64

func QueryOSVBatch(ctx context.Context, queries []OSVPackageQuery) (map[string][]OSVAdvisory, error) {
	results := make(map[string][]OSVAdvisory)
	if len(queries) == 0 {
		return results, nil
	}

	requestQueries := make([]osvBatchQuery, 0, len(queries))
	for _, query := range queries {
		name := strings.TrimSpace(query.Name)
		ecosystem := strings.TrimSpace(query.Ecosystem)
		if name == "" || ecosystem == "" {
			continue
		}
		requestQueries = append(requestQueries, osvBatchQuery{
			Package: osvPackage{Name: name, Ecosystem: ecosystem},
			Version: strings.TrimSpace(query.Version),
		})
	}

	if len(requestQueries) == 0 {
		return results, nil
	}

	for offset := 0; offset < len(requestQueries); offset += osvBatchChunkSize {
		end := offset + osvBatchChunkSize
		if end > len(requestQueries) {
			end = len(requestQueries)
		}
		chunk := requestQueries[offset:end]

		chunkResults, err := queryOSVBatchChunk(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for key, advisories := range chunkResults {
			if len(advisories) > 0 {
				results[key] = advisories
			}
		}
	}

	return results, nil
}

func queryOSVBatchChunk(ctx context.Context, chunk []osvBatchQuery) (map[string][]OSVAdvisory, error) {
	results := make(map[string][]OSVAdvisory)
	if len(chunk) == 0 {
		return results, nil
	}

	payload, err := json.Marshal(osvBatchRequest{Queries: chunk})
	if err != nil {
		return nil, fmt.Errorf("marshal osv batch request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvQueryBatchURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("create osv batch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute osv batch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv batch request failed with status %d", resp.StatusCode)
	}

	var batchResp osvBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("decode osv batch response: %w", err)
	}

	for idx, result := range batchResp.Results {
		if idx >= len(chunk) {
			break
		}
		query := chunk[idx]
		key := OSVPackageKey(query.Package.Ecosystem, query.Package.Name, query.Version)
		if len(result.Vulns) == 0 {
			continue
		}

		items := make([]OSVAdvisory, 0, len(result.Vulns))
		for _, vuln := range result.Vulns {
			referenceURL := ""
			for _, ref := range vuln.References {
				if strings.TrimSpace(ref.Url) != "" {
					referenceURL = strings.TrimSpace(ref.Url)
					break
				}
			}

			fixedVersion := ""
			for _, affected := range vuln.Affected {
				for _, rangeRow := range affected.Ranges {
					for _, event := range rangeRow.Events {
						if strings.TrimSpace(event.Fixed) != "" {
							fixedVersion = strings.TrimSpace(event.Fixed)
							break
						}
					}
					if fixedVersion != "" {
						break
					}
				}
				if fixedVersion != "" {
					break
				}
			}

			severity := strings.TrimSpace(vuln.DatabaseSpecific.Severity)
			if severity == "" && len(vuln.Severity) > 0 {
				severity = strings.TrimSpace(vuln.Severity[0].Score)
			}

			items = append(items, OSVAdvisory{
				ID:           strings.TrimSpace(vuln.ID),
				Aliases:      cleanStringList(vuln.Aliases),
				Summary:      strings.TrimSpace(vuln.Summary),
				Details:      strings.TrimSpace(vuln.Details),
				Severity:     strings.ToLower(severity),
				FixedVersion: fixedVersion,
				ReferenceURL: referenceURL,
			})
		}

		results[key] = items
	}

	return results, nil
}

type osvBatchRequest struct {
	Queries []osvBatchQuery `json:"queries"`
}

type osvBatchQuery struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version,omitempty"`
}

type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

type osvBatchResponse struct {
	Results []osvBatchResult `json:"results"`
}

type osvBatchResult struct {
	Vulns []osvVuln `json:"vulns"`
}

type osvVuln struct {
	ID       string   `json:"id"`
	Aliases  []string `json:"aliases"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	DatabaseSpecific struct {
		Severity string `json:"severity"`
	} `json:"database_specific"`
	References []struct {
		Type string `json:"type"`
		Url  string `json:"url"`
	} `json:"references"`
	Affected []struct {
		Ranges []struct {
			Events []struct {
				Fixed string `json:"fixed"`
			} `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
}

func OSVPackageKey(ecosystem, name, version string) string {
	return strings.ToLower(strings.TrimSpace(ecosystem) + "|" + strings.TrimSpace(name) + "|" + strings.TrimSpace(version))
}

func cleanStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, item)
	}
	if len(cleaned) == 0 {
		return []string{}
	}
	return cleaned
}
