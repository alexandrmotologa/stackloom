package prober

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alexandrmotologa/stackloom/internal/compose"
)

// EndpointProbe records the result of probing a published service port.
type EndpointProbe struct {
	Service    string `json:"service"`
	Port       int    `json:"port"`
	URL        string `json:"url"`
	IsHTTP     bool   `json:"is_http"`
	StatusCode int    `json:"status_code,omitempty"`
	StatusText string `json:"status_text,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
	Live       bool   `json:"live"`
}

// ProbePublishedEndpoints queries all published host ports across services in parallel.
func ProbePublishedEndpoints(ctx context.Context, cfg *compose.ComposeConfig) []EndpointProbe {
	if cfg == nil {
		return nil
	}

	var targets []struct {
		service string
		port    int
	}

	for name, s := range cfg.Services {
		for _, p := range s.Ports {
			if p.Published > 0 {
				targets = append(targets, struct {
					service string
					port    int
				}{service: name, port: p.Published})
			}
		}
	}

	results := make([]EndpointProbe, len(targets))
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 800 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects, report redirect status
		},
	}

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, service string, port int) {
			defer wg.Done()
			url := fmt.Sprintf("http://127.0.0.1:%d", port)
			probe := EndpointProbe{
				Service: service,
				Port:    port,
				URL:     fmt.Sprintf("http://localhost:%d", port),
			}

			start := time.Now()
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				results[idx] = probe
				return
			}

			resp, err := client.Do(req)
			latency := time.Since(start).Milliseconds()
			probe.LatencyMs = latency

			if err == nil {
				probe.Live = true
				probe.IsHTTP = true
				probe.StatusCode = resp.StatusCode
				probe.StatusText = resp.Status
				_ = resp.Body.Close()
			} else {
				probe.Live = false
			}

			results[idx] = probe
		}(i, t.service, t.port)
	}

	wg.Wait()
	return results
}
