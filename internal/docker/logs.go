package docker

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StreamLogs reads and demultiplexes stdout and stderr streams for a container.
func (c *Client) StreamLogs(ctx context.Context, containerID string, tail int, follow bool) (<-chan LogLine, <-chan error) {
	linesCh := make(chan LogLine, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(linesCh)
		defer close(errCh)

		tailParam := "all"
		if tail > 0 {
			tailParam = fmt.Sprintf("%d", tail)
		}

		followParam := "0"
		if follow {
			followParam = "1"
		}

		url := fmt.Sprintf("%s/v1.41/containers/%s/logs?stdout=1&stderr=1&timestamps=1&tail=%s&follow=%s",
			c.baseURL, containerID, tailParam, followParam)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			errCh <- err
			return
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("failed to stream logs (status %d): %s", resp.StatusCode, string(body))
			return
		}

		reader := bufio.NewReader(resp.Body)
		header := make([]byte, 8)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Read 8-byte multiplex header
			_, err := io.ReadFull(reader, header)
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return
				}
				errCh <- err
				return
			}

			streamType := StreamStdout
			switch header[0] {
			case 1:
				streamType = StreamStdout
			case 2:
				streamType = StreamStderr
			}

			payloadSize := binary.BigEndian.Uint32(header[4:8])
			if payloadSize == 0 {
				continue
			}

			payload := make([]byte, payloadSize)
			_, err = io.ReadFull(reader, payload)
			if err != nil {
				if err == io.EOF {
					return
				}
				errCh <- err
				return
			}

			// Parse message line
			msg := string(payload)
			msg = strings.TrimRight(msg, "\r\n")

			ts := time.Now()
			// Extract RFC3339 timestamp prefix if present
			parts := strings.SplitN(msg, " ", 2)
			if len(parts) == 2 {
				if parsedTs, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
					ts = parsedTs
					msg = parts[1]
				} else if parsedTs, err := time.Parse(time.RFC3339, parts[0]); err == nil {
					ts = parsedTs
					msg = parts[1]
				}
			}

			linesCh <- LogLine{
				Stream:    streamType,
				Timestamp: ts,
				Message:   msg,
			}
		}
	}()

	return linesCh, errCh
}
