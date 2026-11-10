//go:build windows

package docker

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

// DefaultDockerHost on Windows points to standard Docker Engine named pipe.
const DefaultDockerHost = "npipe:////./pipe/docker_engine"

func createHTTPClient(rawHost string) (*http.Client, string, error) {
	if rawHost == "" {
		rawHost = DefaultDockerHost
	}

	if strings.HasPrefix(rawHost, "tcp://") || strings.HasPrefix(rawHost, "http://") {
		parsed, err := url.Parse(rawHost)
		if err != nil {
			return nil, "", err
		}
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		baseURL := "http://" + parsed.Host
		return client, baseURL, nil
	}

	if strings.HasPrefix(rawHost, "unix://") {
		socketPath := strings.TrimPrefix(rawHost, "unix://")
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		}
		return &http.Client{Transport: transport, Timeout: 10 * time.Second}, "http://localhost", nil
	}

	// Named pipe transport
	pipePath := rawHost
	if strings.HasPrefix(pipePath, "npipe://") {
		pipePath = strings.TrimPrefix(pipePath, "npipe://")
	}
	pipePath = strings.ReplaceAll(pipePath, "/", "\\")

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return winio.DialPipeContext(ctx, pipePath)
		},
	}

	return &http.Client{Transport: transport, Timeout: 10 * time.Second}, "http://localhost", nil
}
