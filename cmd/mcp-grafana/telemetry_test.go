//go:build unit

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the actual executable: an in-process test alone would miss remote
// logging installed by main, dependency initialization, or shutdown exports.
func TestNoRemoteTelemetryProcess(t *testing.T) {
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancelBuild()
	binaryName := "mcp-grafana"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), binaryName)
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	build.Env = append(os.Environ(), "GOTELEMETRY=off")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build MCP: %s", output)

	grafana := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/privacy-delay" {
			// Make the slow-request assertion independent of the platform's
			// clock resolution; fast navigation calls may take zero ticks.
			time.Sleep(30 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{}`)
			return
		}
		if r.URL.Path == "/api/frontend/settings" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"appUrl":%q}`, "http://"+r.Host)
			return
		}
		http.NotFound(w, r)
	}))
	defer grafana.Close()

	for _, mode := range []string{"generic", "signals", "combined", "invalid"} {
		t.Run("stdio/"+mode, func(t *testing.T) {
			collector, connections := telemetryCollectorTrap(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			serverAddr, metricsAddr := unusedTelemetryTestAddress(t), unusedTelemetryTestAddress(t)
			cmd := exec.CommandContext(ctx, binary,
				"--transport", "stdio", "--enabled-tools", "navigation,api", "--disable-proxied",
				"--debug", "--log-level", "debug", "--slow-request-threshold", "1ns",
				"--metrics", "--address", serverAddr, "--metrics-address", metricsAddr, "--usage-stats", "enabled")
			cmd.Env = telemetryProcessEnv(grafana.URL, collector, mode)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdin, err := cmd.StdinPipe()
			require.NoError(t, err)
			stdout, err := cmd.StdoutPipe()
			require.NoError(t, err)
			require.NoError(t, cmd.Start())
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
				if t.Failed() {
					t.Log(stderr.String())
				}
			})

			messages := make(chan string)
			go func() {
				defer close(messages)
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					select {
					case messages <- scanner.Text():
					case <-ctx.Done():
						return
					}
				}
			}()
			encoder := json.NewEncoder(stdin)
			request := func(id int, method string, params any) json.RawMessage {
				t.Helper()
				require.NoError(t, encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}))
				for {
					select {
					case line, ok := <-messages:
						require.True(t, ok, "MCP stdout closed before response %d", id)
						var response struct {
							ID     int             `json:"id"`
							Result json.RawMessage `json:"result"`
							Error  json.RawMessage `json:"error"`
						}
						require.NoError(t, json.Unmarshal([]byte(line), &response))
						if response.ID != id {
							continue
						}
						require.Empty(t, response.Error, "JSON-RPC error: %s", response.Error)
						return response.Result
					case <-ctx.Done():
						t.Fatal("timed out waiting for MCP response")
					}
				}
			}
			request(1, "initialize", map[string]any{
				"protocolVersion": "2025-11-25",
				"capabilities":    map[string]any{},
				"clientInfo":      map[string]string{"name": "privacy-test", "version": "1"},
			})
			require.NoError(t, encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}))
			assert.Contains(t, string(request(2, "tools/list", map[string]any{})), "generate_deeplink")
			result := request(3, "tools/call", map[string]any{
				"name": "generate_deeplink", "arguments": map[string]string{"resourceType": "dashboard", "dashboardUid": "privacy-test"},
				"_meta": map[string]string{"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
			})
			assert.Contains(t, string(result), "/d/privacy-test")
			failed := request(4, "tools/call", map[string]any{
				"name": "generate_deeplink", "arguments": map[string]string{"resourceType": "invalid"},
			})
			assert.Contains(t, string(failed), `"isError":true`)
			request(5, "tools/call", map[string]any{
				"name": "grafana_api_request", "arguments": map[string]string{"endpoint": "/api/privacy-delay"},
			})

			// Stdio must ignore HTTP/metrics addresses even with --metrics set.
			for _, addr := range []string{serverAddr, metricsAddr} {
				listener, err := net.Listen("tcp", addr)
				require.NoError(t, err, "stdio opened a listener at %s", addr)
				_ = listener.Close()
			}
			// Export batch delays are set to 1ms in the child. Keep it alive
			// through several intervals, then exercise graceful EOF shutdown.
			time.Sleep(150 * time.Millisecond)
			require.NoError(t, stdin.Close())
			require.NoError(t, cmd.Wait())
			assert.Zero(t, connections.Load(), "MCP connected to a telemetry collector")
			assert.Contains(t, stderr.String(), "Starting Grafana MCP server using stdio")
			assert.Contains(t, stderr.String(), "level=DEBUG")
			assert.Contains(t, stderr.String(), "Slow request")
			assert.NotContains(t, stderr.String(), "Starting metrics server")
			assert.NotContains(t, stderr.String(), "OTLP log export configured")
		})
	}

	for _, mode := range []string{"disabled", "main", "separate"} {
		t.Run("http-metrics/"+mode, func(t *testing.T) {
			collector, connections := telemetryCollectorTrap(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			addr := unusedTelemetryTestAddress(t)
			metricsAddr := addr
			args := []string{"--transport", "streamable-http", "--address", addr, "--enabled-tools", "navigation", "--disable-proxied"}
			if mode != "disabled" {
				args = append(args, "--metrics")
			}
			if mode == "separate" {
				metricsAddr = unusedTelemetryTestAddress(t)
				args = append(args, "--metrics-address", metricsAddr)
			}
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Env = telemetryProcessEnv(grafana.URL, collector, "combined")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			require.NoError(t, cmd.Start())
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				if t.Failed() {
					t.Log(stderr.String())
				}
			})
			client := &http.Client{Timeout: time.Second}
			var response *http.Response
			require.Eventually(t, func() bool {
				var err error
				response, err = client.Get("http://" + metricsAddr + "/metrics")
				return err == nil
			}, 10*time.Second, 25*time.Millisecond)
			defer response.Body.Close()
			if mode == "disabled" {
				assert.Equal(t, http.StatusNotFound, response.StatusCode)
			} else {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				assert.Contains(t, string(body), "go_")
			}
			time.Sleep(150 * time.Millisecond)
			assert.Zero(t, connections.Load(), "MCP connected to a telemetry collector")
		})
	}
}

func telemetryCollectorTrap(t *testing.T) (string, *atomic.Int64) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	connections := new(atomic.Int64)
	var workers sync.WaitGroup
	workers.Go(func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			_ = connection.Close()
		}
	})
	t.Cleanup(func() {
		_ = listener.Close()
		workers.Wait()
	})
	return "http://" + listener.Addr().String(), connections
}

func unusedTelemetryTestAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return addr
}

func telemetryProcessEnv(grafanaURL, collector, mode string) []string {
	var env []string
	for _, entry := range os.Environ() {
		upper := strings.ToUpper(entry)
		if !strings.HasPrefix(upper, "OTEL_") && !strings.HasPrefix(upper, "GRAFANA_") {
			env = append(env, entry)
		}
	}
	env = append(env, "GRAFANA_URL="+grafanaURL, "GRAFANA_SERVICE_ACCOUNT_TOKEN=privacy-test-token",
		"GRAFANA_USAGE_STATS=enabled", "GRAFANA_USAGE_STATS_ENDPOINT="+collector,
		"GOTELEMETRY=off", "OTEL_TRACES_EXPORTER=otlp", "OTEL_LOGS_EXPORTER=otlp", "OTEL_METRICS_EXPORTER=otlp",
		"OTEL_TRACES_SAMPLER=always_on", "OTEL_BSP_SCHEDULE_DELAY=1", "OTEL_BLRP_SCHEDULE_DELAY=1")
	if mode == "invalid" {
		collector = "://invalid-collector"
	}
	for _, signal := range []string{"", "_TRACES", "_LOGS", "_METRICS"} {
		if (mode == "generic" && signal != "") || (mode == "signals" && signal == "") {
			continue
		}
		prefix := "OTEL_EXPORTER_OTLP" + signal
		env = append(env, prefix+"_ENDPOINT="+collector, prefix+"_INSECURE=true", prefix+"_TIMEOUT=100")
		if mode == "invalid" {
			env = append(env, prefix+"_CERTIFICATE=/missing/collector.pem")
		}
	}
	return env
}
