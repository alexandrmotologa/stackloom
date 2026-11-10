//go:build !windows

package docker

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultDockerHost on Unix points to standard Docker Unix domain socket.
const DefaultDockerHost = "unix:///var/run/docker.sock"

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

	socketPath := strings.TrimPrefix(rawHost, "unix://")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}

	return &http.Client{Transport: transport, Timeout: 10 * time.Second}, "http://localhost", nil
}
