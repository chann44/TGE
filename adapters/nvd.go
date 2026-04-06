package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NVDAdvisory struct {
	CVEID        string
	Summary      string
	Severity     string
	ReferenceURL string
}

type nvdResponse struct {
	Vulnerabilities []struct {
		CVE struct {
			ID           string `json:"id"`
			Descriptions []struct {
				Lang  string `json:"lang"`
				Value string `json:"value"`
			} `json:"descriptions"`
			References []struct {
				URL string `json:"url"`
			} `json:"references"`
			Metrics struct {
				CVSSMetricV31 []struct {
					CVSSData struct {
						BaseSeverity string `json:"baseSeverity"`
					} `json:"cvssData"`
				} `json:"cvssMetricV31"`
				CVSSMetricV30 []struct {
					CVSSData struct {
						BaseSeverity string `json:"baseSeverity"`
					} `json:"cvssData"`
				} `json:"cvssMetricV30"`
				CVSSMetricV2 []struct {
					BaseSeverity string `json:"baseSeverity"`
				} `json:"cvssMetricV2"`
			} `json:"metrics"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

func GetNVDAdvisory(ctx context.Context, cveID, apiKey string) (*NVDAdvisory, error) {
	id := strings.ToUpper(strings.TrimSpace(cveID))
	if id == "" {
		return nil, fmt.Errorf("cve id is required")
	}

	endpoint := "https://services.nvd.nist.gov/rest/json/cves/2.0?cveId=" + url.QueryEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create nvd advisory request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("apiKey", strings.TrimSpace(apiKey))
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute nvd advisory request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nvd advisory request failed with status %d", resp.StatusCode)
	}

	var payload nvdResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode nvd advisory response: %w", err)
	}
	if len(payload.Vulnerabilities) == 0 {
		return nil, fmt.Errorf("nvd advisory not found")
	}

	cve := payload.Vulnerabilities[0].CVE
	summary := ""
	for _, description := range cve.Descriptions {
		if strings.EqualFold(strings.TrimSpace(description.Lang), "en") && strings.TrimSpace(description.Value) != "" {
			summary = strings.TrimSpace(description.Value)
			break
		}
	}
	if summary == "" && len(cve.Descriptions) > 0 {
		summary = strings.TrimSpace(cve.Descriptions[0].Value)
	}

	severity := ""
	if len(cve.Metrics.CVSSMetricV31) > 0 {
		severity = strings.ToLower(strings.TrimSpace(cve.Metrics.CVSSMetricV31[0].CVSSData.BaseSeverity))
	}
	if severity == "" && len(cve.Metrics.CVSSMetricV30) > 0 {
		severity = strings.ToLower(strings.TrimSpace(cve.Metrics.CVSSMetricV30[0].CVSSData.BaseSeverity))
	}
	if severity == "" && len(cve.Metrics.CVSSMetricV2) > 0 {
		severity = strings.ToLower(strings.TrimSpace(cve.Metrics.CVSSMetricV2[0].BaseSeverity))
	}

	referenceURL := ""
	if len(cve.References) > 0 {
		referenceURL = strings.TrimSpace(cve.References[0].URL)
	}

	return &NVDAdvisory{
		CVEID:        firstNonEmptyTrimmed(cve.ID, id),
		Summary:      summary,
		Severity:     severity,
		ReferenceURL: referenceURL,
	}, nil
}
