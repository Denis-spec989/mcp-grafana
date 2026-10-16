//go:build unit

package mcpgrafana

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestNoRemoteTelemetryImports is also run as an explicit CI policy check.
// Check all application sources, rather than only observability, so an exporter
// cannot accidentally be reintroduced through a tool or a new startup path.
func TestNoRemoteTelemetryImports(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository sources")
	}
	root := filepath.Dir(sourceFile)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "dist" || entry.Name() == "build") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			// The Prometheus exporter only serves local pull metrics. Other
			// exporters and export-capable log/trace providers are prohibited.
			if strings.HasPrefix(importPath, "go.opentelemetry.io/otel/exporters/") {
				if importPath != "go.opentelemetry.io/otel/exporters/prometheus" {
					t.Errorf("%s: remote telemetry import %q", fset.Position(spec.Pos()), importPath)
				}
			}
			for _, blocked := range []string{
				"go.opentelemetry.io/otel/sdk/log",
				"go.opentelemetry.io/otel/sdk/trace",
				"go.opentelemetry.io/contrib/bridges/otelslog",
				"go.opentelemetry.io/contrib/exporters/",
				"go.opentelemetry.io/auto",
				"github.com/prometheus/client_golang/prometheus/push",
				"github.com/getsentry/sentry-go",
			} {
				if importPath == blocked || strings.HasPrefix(importPath, strings.TrimSuffix(blocked, "/")+"/") {
					t.Errorf("%s: remote telemetry import %q", fset.Position(spec.Pos()), importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
