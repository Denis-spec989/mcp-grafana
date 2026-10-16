---
title: Observability (local metrics and logs)
menuTitle: Observability
description: Expose local Prometheus metrics and diagnostic logs without remote telemetry export.
keywords:
  - Prometheus
  - metrics
  - OpenTelemetry
  - logs
  - MCP
weight: 2
aliases: []
---

# Observability (local metrics and logs)

The MCP server provides local stderr logs and optional Prometheus metrics following the [OTel MCP semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/mcp/). It does not send its own logs, traces, usage events, or metrics to telemetry collectors, including during startup and shutdown.

## Enable Prometheus metrics

Metrics are disabled by default and require the SSE or streamable HTTP transport:

```bash
# Metrics on the main server at /metrics
./mcp-grafana -t streamable-http --metrics

# Metrics on a separate loopback address
./mcp-grafana -t streamable-http --metrics --metrics-address 127.0.0.1:9090
```

| Metric | Type | Description |
|--------|------|-------------|
| `mcp_server_operation_duration_seconds` | Histogram | MCP operation duration (labels: `mcp_method_name`, `gen_ai_tool_name`, `error_type`, `network_transport`, `mcp_protocol_version`) |
| `mcp_server_session_duration_seconds` | Histogram | MCP client session duration (labels: `network_transport`, `mcp_protocol_version`) |
| `http_server_request_duration_seconds` | Histogram | HTTP server request duration (from otelhttp) |

Metrics are returned when a client scrapes `/metrics`; the MCP server does not push them to a collector. Anyone with network access to that endpoint can scrape it. Use a trusted listen address and restrict network access.

Stdio opens no HTTP or metrics listener, even when `--metrics` or `--metrics-address` is supplied.

## Use local diagnostic logs

Logs are written to stderr. Set `--log-level` to `debug`, `info`, `warn`, or `error`; use `--debug` for detailed Grafana HTTP diagnostics.

Slow-request logging works with every transport and does not require metrics:

```bash
./mcp-grafana -t stdio --slow-request-threshold 500ms
./mcp-grafana -t streamable-http --slow-request-threshold 500ms --slow-request-log-level info
```

The default threshold of `0` disables slow-request events. Events include the MCP method, duration, threshold, tool name when applicable, and error details when a request fails.

## Remote telemetry policy

This build does not configure OTLP trace or log exporters, background export queues, or shutdown export. `OTEL_EXPORTER_OTLP_*` environment variables, including signal-specific traces, logs, and metrics settings, cannot enable telemetry export. There is no flag to enable remote export. OpenTelemetry instrumentation and libraries remain where needed for local monitoring.

The policy applies to the MCP process's own telemetry. Normal tool requests to Grafana, datasource MCP endpoints, and the public plugin catalog remain available. Tool results still go to the MCP client. Your MCP/AI client, Grafana, operating-system agents, or a separate collector reading stderr or `/metrics` can transmit data according to their own configuration. Configure those components separately. Go library consumers that supply their own logger, transport, or OpenTelemetry providers are responsible for those integrations.

## Run with Docker and local metrics

```bash
docker run --rm -p 127.0.0.1:8000:8000 \
  -e GRAFANA_URL=http://grafana:3000 \
  -e GRAFANA_SERVICE_ACCOUNT_TOKEN=<your token> \
  grafana/mcp-grafana \
  -t streamable-http --metrics
```

## Next steps

- [Build, test, and lint](../build-and-test/)
- [Transports and addresses](../../configure/transports-and-addresses/)
- [Command-line flags](../../configure/command-line-flags/)
