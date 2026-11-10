package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/alexandrmotologa/stackloom/internal/api"
	"github.com/alexandrmotologa/stackloom/internal/compose"
	"github.com/alexandrmotologa/stackloom/internal/config"
	"github.com/alexandrmotologa/stackloom/internal/docker"
	"github.com/alexandrmotologa/stackloom/internal/export"
	"github.com/alexandrmotologa/stackloom/internal/fixer"
	"github.com/alexandrmotologa/stackloom/internal/graph"
	"github.com/alexandrmotologa/stackloom/internal/linter"
	"github.com/alexandrmotologa/stackloom/web"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		// If running without arguments, default to "serve"
		runServe(os.Args[1:])
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "serve":
		runServe(args)
	case "lint":
		runLint(args)
	case "graph":
		runGraph(args)
	case "blast-radius":
		runBlastRadius(args)
	case "fix":
		runFix(args)
	case "export":
		runExport(args)
	case "version", "--version", "-v":
		fmt.Printf("StackLoom v%s (< 30MB RAM Docker Compose Topology Studio)\n", version)
	case "help", "--help", "-h":
		printHelp()
	default:
		// Check if first argument is a flag like -p, -f or a filename
		if strings.HasPrefix(command, "-") || strings.HasSuffix(command, ".yml") || strings.HasSuffix(command, ".yaml") {
			runServe(os.Args[1:])
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
			printHelp()
			os.Exit(1)
		}
	}
}

func printHelp() {
	fmt.Println(`StackLoom — Interactive Docker Compose Topology Studio & Pre-Flight Linter

Usage:
  stackloom [command] [options]

Commands:
  serve          Launch the interactive topology studio web dashboard (default)
  lint           Run static pre-flight linter on compose configuration
  graph          Print dependency graph (ASCII, Mermaid, or Graphviz DOT)
  blast-radius   Simulate cascading failure impact of an outage on a service
  fix            Automatically apply suggested YAML overrides to docker-compose.override.yml
  export         Generate a standalone offline HTML interactive report
  version        Show application version

Examples:
  stackloom serve -p 9095 -f docker-compose.yml
  stackloom lint -f docker-compose.yml --format json
  stackloom blast-radius -f docker-compose.yml postgres
  stackloom fix -f docker-compose.yml --auto
  stackloom export -f docker-compose.yml -o topology.html`)
}

type stringSliceFlags []string

func (s *stringSliceFlags) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlags) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func parseComposeFiles(fs *flag.FlagSet, files *stringSliceFlags, fileSingle *string) []string {
	var result []string
	if len(*files) > 0 {
		for _, f := range *files {
			for _, part := range strings.Split(f, ",") {
				clean := strings.TrimSpace(part)
				if clean != "" {
					result = append(result, clean)
				}
			}
		}
	} else if *fileSingle != "" {
		for _, part := range strings.Split(*fileSingle, ",") {
			clean := strings.TrimSpace(part)
			if clean != "" {
				result = append(result, clean)
			}
		}
	}

	// Auto-detect default files if none provided
	if len(result) == 0 {
		candidates := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				result = append(result, c)
				break
			}
		}
	}

	if len(result) == 0 {
		result = []string{"docker-compose.yml"}
	}

	return result
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("p", 9095, "Server port")
	fs.IntVar(port, "port", 9095, "Server port")
	host := fs.String("host", "127.0.0.1", "Bind address")
	fileSingle := fs.String("f", "", "Compose file")
	fs.StringVar(fileSingle, "file", "", "Compose file")
	var filesList stringSliceFlags
	fs.Var(&filesList, "files", "Multiple compose files")
	profilesStr := fs.String("profile", "", "Comma-separated active profiles")
	dockerHost := fs.String("docker-host", "", "Custom Docker host socket")
	noProbe := fs.Bool("no-probe", false, "Disable local host socket port probing")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Flags error: %v\n", err)
		os.Exit(1)
	}

	composeFiles := parseComposeFiles(fs, &filesList, fileSingle)

	var activeProfiles []string
	if *profilesStr != "" {
		for _, p := range strings.Split(*profilesStr, ",") {
			clean := strings.TrimSpace(p)
			if clean != "" {
				activeProfiles = append(activeProfiles, clean)
			}
		}
	}

	cfg := &config.Config{
		Port:             *port,
		Host:             *host,
		ComposeFiles:     composeFiles,
		ActiveProfiles:   activeProfiles,
		DockerHost:       *dockerHost,
		ProbeHostSockets: !*noProbe,
	}

	// Initialize Docker client
	dockerClient, err := docker.NewClient(cfg.DockerHost)
	if err != nil {
		fmt.Printf("Docker client initialization: %v (running in offline mode)\n", err)
	}

	// Initialize API Server
	server, err := api.NewServer(cfg, dockerClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing topology server: %v\n", err)
		os.Exit(1)
	}

	// Get embedded static assets
	staticFS, err := web.GetFileSystem()
	if err != nil {
		fmt.Printf("Warning: Embedded web UI bundle not found: %v\n", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux, staticFS)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Pretty startup banner
	fmt.Println("================================================================================")
	fmt.Printf("  StackLoom v%s: Docker Compose Topology Studio & Pre-Flight Linter\n", version)
	fmt.Println("================================================================================")
	fmt.Printf("  • Web Studio:       http://%s\n", addr)
	fmt.Printf("  • Compose Files:    %s\n", strings.Join(cfg.ComposeFiles, ", "))
	if len(cfg.ActiveProfiles) > 0 {
		fmt.Printf("  • Active Profiles:  %s\n", strings.Join(cfg.ActiveProfiles, ", "))
	}
	if dockerClient != nil && dockerClient.IsConnected() {
		fmt.Printf("  • Docker Socket:    CONNECTED (%s)\n", dockerClient.Host())
	} else {
		fmt.Println("  • Docker Socket:    OFFLINE (Static Topology & Pre-Flight Linter mode)")
	}
	fmt.Println("================================================================================")
	fmt.Println("Press Ctrl+C to stop.")

	// Graceful shutdown handling
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
			os.Exit(1)
		}
	}()

	<-stopCh
	fmt.Println("\nShutting down StackLoom...")
	_ = httpServer.Close()
}

func runLint(args []string) {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	fileSingle := fs.String("f", "", "Compose file")
	fs.StringVar(fileSingle, "file", "", "Compose file")
	var filesList stringSliceFlags
	fs.Var(&filesList, "files", "Multiple compose files")
	format := fs.String("format", "text", "Output format: text or json")
	noProbe := fs.Bool("no-probe", false, "Disable local host socket port probing")
	exitCode := fs.Bool("exit-code", true, "Exit with code 1 if critical issues found")

	_ = fs.Parse(args)
	composeFiles := parseComposeFiles(fs, &filesList, fileSingle)

	parser := compose.NewParser(nil)
	merger := compose.NewMerger()
	var baseConfig *compose.ComposeConfig
	var allUnsets []string
	basePath := filepath.Dir(composeFiles[0])

	for _, file := range composeFiles {
		res, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", file, err)
			os.Exit(1)
		}
		allUnsets = append(allUnsets, res.UnsetEnvs...)
		if baseConfig == nil {
			baseConfig = res.Config
		} else {
			baseConfig = merger.Merge(baseConfig, res.Config)
		}
	}

	g := graph.BuildGraph(baseConfig)
	l := linter.NewLinter(!*noProbe)
	result := l.Lint(baseConfig, g, allUnsets, basePath)

	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	} else {
		fmt.Printf("\nStackLoom Pre-Flight Linter: %d findings (%d critical, %d warnings, %d security)\n\n",
			len(result.Findings), result.CriticalCount, result.WarningCount, result.SecurityCount)

		if len(result.Findings) == 0 {
			fmt.Println("✓ All checks passed! Zero port collisions or dependency errors detected.")
		} else {
			for _, f := range result.Findings {
				var prefix string
				switch f.Severity {
				case linter.SeverityCritical:
					prefix = "[CRITICAL]"
				case linter.SeverityWarning:
					prefix = "[WARNING]"
				case linter.SeveritySecurity:
					prefix = "[SECURITY]"
				}

				svc := ""
				if f.Service != "" {
					svc = fmt.Sprintf(" (%s)", f.Service)
				}
				fmt.Printf("%s %s%s: %s\n", prefix, f.RuleID, svc, f.Message)
				if f.Detail != "" {
					fmt.Printf("   Detail: %s\n", f.Detail)
				}
				fmt.Printf("   Action: %s\n", f.Recommendation)
				if f.SuggestedOverride != "" {
					fmt.Printf("   Suggested Override:\n%s\n", indent(f.SuggestedOverride, "     "))
				}
				fmt.Println()
			}
		}
	}

	if *exitCode && !result.Passed {
		os.Exit(1)
	}
}

func runGraph(args []string) {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	fileSingle := fs.String("f", "", "Compose file")
	fs.StringVar(fileSingle, "file", "", "Compose file")
	var filesList stringSliceFlags
	fs.Var(&filesList, "files", "Multiple compose files")
	format := fs.String("format", "ascii", "Output format: ascii, mermaid, or dot")
	profilesStr := fs.String("profile", "", "Comma-separated active profiles")

	_ = fs.Parse(args)
	composeFiles := parseComposeFiles(fs, &filesList, fileSingle)

	parser := compose.NewParser(nil)
	merger := compose.NewMerger()
	var baseConfig *compose.ComposeConfig

	for _, file := range composeFiles {
		res, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", file, err)
			os.Exit(1)
		}
		if baseConfig == nil {
			baseConfig = res.Config
		} else {
			baseConfig = merger.Merge(baseConfig, res.Config)
		}
	}

	var activeProfiles []string
	if *profilesStr != "" {
		for _, p := range strings.Split(*profilesStr, ",") {
			clean := strings.TrimSpace(p)
			if clean != "" {
				activeProfiles = append(activeProfiles, clean)
			}
		}
	}

	filtered := merger.FilterByProfiles(baseConfig, activeProfiles)
	g := graph.BuildGraph(filtered)

	switch strings.ToLower(*format) {
	case "mermaid":
		fmt.Println(g.ToMermaid())
	case "dot":
		fmt.Println(g.ToDOT())
	default:
		plan, err := graph.ComputeStartupPlan(g)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Graph ordering error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(plan.ToASCII())
	}
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	fileSingle := fs.String("f", "", "Compose file")
	fs.StringVar(fileSingle, "file", "", "Compose file")
	var filesList stringSliceFlags
	fs.Var(&filesList, "files", "Multiple compose files")
	outputPath := fs.String("o", "stackloom-topology.html", "Output HTML file path")
	fs.StringVar(outputPath, "output", "stackloom-topology.html", "Output HTML file path")

	_ = fs.Parse(args)
	composeFiles := parseComposeFiles(fs, &filesList, fileSingle)

	parser := compose.NewParser(nil)
	merger := compose.NewMerger()
	var baseConfig *compose.ComposeConfig
	var allUnsets []string
	basePath := filepath.Dir(composeFiles[0])

	for _, file := range composeFiles {
		res, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", file, err)
			os.Exit(1)
		}
		allUnsets = append(allUnsets, res.UnsetEnvs...)
		if baseConfig == nil {
			baseConfig = res.Config
		} else {
			baseConfig = merger.Merge(baseConfig, res.Config)
		}
	}

	g := graph.BuildGraph(baseConfig)
	plan, _ := graph.ComputeStartupPlan(g)
	l := linter.NewLinter(true)
	result := l.Lint(baseConfig, g, allUnsets, basePath)

	htmlContent, err := export.GenerateHTML(baseConfig, g, plan, result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Export failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outputPath, []byte(htmlContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Standalone interactive HTML report saved to %s\n", *outputPath)
}

func runBlastRadius(args []string) {
	fs := flag.NewFlagSet("blast-radius", flag.ExitOnError)
	fileSingle := fs.String("f", "", "Compose file")
	fs.StringVar(fileSingle, "file", "", "Compose file")
	var filesList stringSliceFlags
	fs.Var(&filesList, "files", "Multiple compose files")

	_ = fs.Parse(args)
	nonFlagArgs := fs.Args()
	if len(nonFlagArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: specify target service name to simulate outage (e.g. stackloom blast-radius -f docker-compose.yml postgres)\n")
		os.Exit(1)
	}
	targetService := nonFlagArgs[0]

	composeFiles := parseComposeFiles(fs, &filesList, fileSingle)
	parser := compose.NewParser(nil)
	merger := compose.NewMerger()
	var baseConfig *compose.ComposeConfig

	for _, file := range composeFiles {
		res, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", file, err)
			os.Exit(1)
		}
		if baseConfig == nil {
			baseConfig = res.Config
		} else {
			baseConfig = merger.Merge(baseConfig, res.Config)
		}
	}

	g := graph.BuildGraph(baseConfig)
	report := g.CalculateBlastRadius(targetService)

	fmt.Printf("\nOutage Simulation: Failure of service %q\n", targetService)
	fmt.Printf("Cluster Blast Radius: %d of %d services affected (%.1f%% of cluster)\n\n",
		report.AffectedCount, report.TotalServices, report.BlastRadiusRatio*100)

	if len(report.ImpactTree) == 0 {
		fmt.Println("✓ Zero cascading impact: no other services depend on this service.")
	} else {
		fmt.Println("Cascading Failure Impact Tree:")
		currentDepth := 0
		for _, node := range report.ImpactTree {
			if node.Depth != currentDepth {
				currentDepth = node.Depth
				fmt.Printf("  ├── Cascade Level %d:\n", currentDepth)
			}
			sevLabel := "[DEGRADED]"
			if node.Severity == graph.SeverityFatal {
				sevLabel = "[FATAL CRASH]"
			}
			fmt.Printf("  │    ├── %s %s (awaits %s via %s)\n", node.Service, sevLabel, node.DirectCause, node.Condition)
		}
	}

	if len(report.Unaffected) > 0 {
		fmt.Printf("\nUnaffected Services:\n  • %s\n", strings.Join(report.Unaffected, ", "))
	}
	fmt.Println()
}

func runFix(args []string) {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	fileSingle := fs.String("f", "", "Compose file")
	fs.StringVar(fileSingle, "file", "", "Compose file")
	var filesList stringSliceFlags
	fs.Var(&filesList, "files", "Multiple compose files")
	auto := fs.Bool("auto", false, "Apply all fixes automatically without interactive prompt")

	_ = fs.Parse(args)
	composeFiles := parseComposeFiles(fs, &filesList, fileSingle)
	parser := compose.NewParser(nil)
	merger := compose.NewMerger()
	var baseConfig *compose.ComposeConfig
	var allUnsets []string
	basePath := filepath.Dir(composeFiles[0])

	for _, file := range composeFiles {
		res, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", file, err)
			os.Exit(1)
		}
		allUnsets = append(allUnsets, res.UnsetEnvs...)
		if baseConfig == nil {
			baseConfig = res.Config
		} else {
			baseConfig = merger.Merge(baseConfig, res.Config)
		}
	}

	g := graph.BuildGraph(baseConfig)
	l := linter.NewLinter(true)
	result := l.Lint(baseConfig, g, allUnsets, basePath)

	var fixable []linter.Finding
	for _, f := range result.Findings {
		if f.SuggestedOverride != "" {
			fixable = append(fixable, f)
		}
	}

	if len(fixable) == 0 {
		fmt.Println("✓ No automated overrides needed. All checks passed or no fixable rules triggered.")
		return
	}

	fmt.Printf("\nFound %d fixable finding(s):\n", len(fixable))
	autoFixer := fixer.NewAutoFixer(basePath)
	appliedCount := 0

	for i, f := range fixable {
		fmt.Printf("\n[%d/%d] %s (%s): %s\n", i+1, len(fixable), f.RuleID, f.Service, f.Message)
		fmt.Printf("Suggested YAML:\n%s\n", indent(f.SuggestedOverride, "  "))

		apply := *auto
		if !apply {
			fmt.Print("Apply this fix to docker-compose.override.yml? [y/N]: ")
			var response string
			_, _ = fmt.Scanln(&response)
			response = strings.TrimSpace(strings.ToLower(response))
			if response == "y" || response == "yes" {
				apply = true
			}
		}

		if apply {
			msg, err := autoFixer.ApplyOverride(f.SuggestedOverride)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying override: %v\n", err)
			} else {
				fmt.Printf("✓ %s\n", msg)
				appliedCount++
			}
		}
	}

	fmt.Printf("\nFinished: applied %d override(s) to %s\n", appliedCount, autoFixer.OverrideFilePath())
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
