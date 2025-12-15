package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/analyzer"
	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/generator"
	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/model"
	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/search"
)

//go:embed registry.json
var registryData []byte

func main() {
	// Subcommands
	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	buildDebug := buildCmd.Bool("debug", false, "Enable verbose output")
	buildOutput := buildCmd.String("output", "", "Path to the 'nexus/generated' directory")
	buildCatalogOnly := buildCmd.Bool("catalog-only", false, "Only update catalog, do not generate code")

	searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
	searchParam := searchCmd.String("search-param", "", "Search service by parameter name")
	searchDebug := searchCmd.Bool("debug", false, "Enable verbose output")

	dumpCmd := flag.NewFlagSet("dump-catalog", flag.ExitOnError)
	dumpDebug := dumpCmd.Bool("debug", false, "Enable verbose output")

	if len(os.Args) < 2 {
		fmt.Println("Usage: nexus-cli <command> [arguments]")
		fmt.Println("Commands: build, search, dump-catalog")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd.Parse(os.Args[2:])
		runBuild(*buildDebug, *buildOutput, *buildCatalogOnly)
	case "search":
		searchCmd.Parse(os.Args[2:])
		runSearch(*searchParam, *searchDebug)
	case "dump-catalog":
		dumpCmd.Parse(os.Args[2:])
		runDump(*dumpDebug)
	default:
		// Smart-Run search?
		if strings.HasPrefix(os.Args[1], "-") {
			searchCmd.Parse(os.Args[1:])
			runSearch(*searchParam, *searchDebug)
		} else {
			fmt.Println("Unknown command. Expected 'build', 'search', or 'dump-catalog'.")
			os.Exit(1)
		}
	}
}

func runDump(debug bool) {
	path := search.ResolveDefaultCatalog()
	if debug {
		fmt.Printf("Reading catalog from: %s\n", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading catalog: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// --- Search Logic ---

func runSearch(query string, debug bool) {
	// 1. Resolve Catalog Path
	catalogPath := search.ResolveDefaultCatalog()
	if debug {
		fmt.Printf("DEBUG: Using catalog path: %s\n", catalogPath)
	}

	// 2. Auto-Discovery Check
	catalog, err := search.LoadCatalog(catalogPath)
	if err != nil {
		fmt.Println("Catalog not found or invalid. Running auto-discovery...")
		runBuild(debug, "", false) // Propagate debug, no output override, full build
		// Re-read
		catalog, err = search.LoadCatalog(catalogPath)
		if err != nil {
			fmt.Printf("Error: Could not build catalog: %v\n", err)
			os.Exit(1)
		}
	}

	if debug {
		fmt.Printf("DEBUG: Catalog loaded. %d services found.\n", len(catalog.Services))
	}

	// 4. Search Execution
	if query != "" {
		if debug {
			fmt.Printf("DEBUG: Searching for param '%s'...\n", query)
		}
		results := search.SearchByParam(catalog, query)
		if len(results) == 0 {
			fmt.Println("No services found with that parameter.")
		} else {
			fmt.Printf("Found %d services with parameter '%s':\n", len(results), query)
			for _, res := range results {
				fmt.Printf("- %s.%s\n", res.Namespace, res.Method)
				fmt.Printf("  Match: %s (%s)\n", res.MatchedParam, res.ParamType)
				if res.Description != "" {
					fmt.Printf("  Description: %s\n", res.Description)
				}
			}
		}
	} else {
		// List all by default
		fmt.Println("Available Services:")
		for _, s := range catalog.Services {
			fmt.Printf("- %s.%s\n  %s\n", s.Namespace, s.Method, s.Description)
			if len(s.Inputs) > 0 {
				fmt.Println("  Inputs:")
				for _, in := range s.Inputs {
					fmt.Printf("    - %s (%s)\n", in.Name, in.Type)
				}
			}
			if len(s.Outputs) > 0 {
				fmt.Println("  Outputs:")
				for _, out := range s.Outputs {
					fmt.Printf("    - %s (%s)\n", out.Name, out.Type)
				}
			}
		}
	}
}

// --- Path Resolution Logic ---

func resolveOutputDir(flagPath string) (string, error) {
	// 1. Explicit Flag
	if flagPath != "" {
		if _, err := os.Stat(flagPath); os.IsNotExist(err) {
			if err := os.MkdirAll(flagPath, 0755); err != nil {
				return "", fmt.Errorf("could not create output dir %s: %v", flagPath, err)
			}
		}
		path, _ := filepath.Abs(flagPath)
		return path, nil
	}

	// 2. Default: Create 'generated' in Current Working Directory
	cwd, _ := os.Getwd()
	target := filepath.Join(cwd, "generated")

	if _, err := os.Stat(target); os.IsNotExist(err) {
		if err := os.MkdirAll(target, 0755); err != nil {
			return "", fmt.Errorf("could not create output dir %s: %v", target, err)
		}
	}

	return target, nil
}

// --- Build / Crawler Logic ---

func runBuild(debug bool, outputFlag string, catalogOnly bool) {
	fmt.Println("Starting Nexus Library Discovery (DDD Mode)...")

	// Create Temp Dir
	tempDir, err := os.MkdirTemp("", "nexus-build")
	if err != nil {
		log.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if debug {
		fmt.Printf("DEBUG: Temp build dir: %s\n", tempDir)
	}

	execCmd(tempDir, "go", "mod", "init", "nexus-temp-builder")

	var libraries []string
	if err := json.Unmarshal(registryData, &libraries); err != nil {
		log.Fatalf("Error parsing internal registry: %v", err)
	}

	var catalog model.Catalog
	var allMetadata []model.FunctionMetadata

	for _, lib := range libraries {
		fmt.Printf("Checking library: %s (@develop) ... ", lib)

		// 1. Ensure Installed (FORCE @develop)
		// NOTE: In production this should come from registry.json metadata
		if err := analyzer.EnsureLibraryInstalled(tempDir, lib, "develop", debug); err != nil {
			fmt.Printf("Failed: %v\n", err)
			continue
		}

		// 2. Resolve Root Path
		rootPath, err := analyzer.ResolvePackagePath(tempDir, lib, debug)
		if err != nil {
			fmt.Printf("Error resolving path: %v\n", err)
			continue
		}
		if debug {
			fmt.Printf("DEBUG: Root path for %s: %s\n", lib, rootPath)
		} else {
			fmt.Println("OK")
		}

		// 3. Crawl Recursively
		// Simplify namespace: github.com/japablazatww/libreria-a -> libreria-a
		baseNamespace := filepath.Base(lib)
		baseImportPath := lib
		analyzer.CrawlLibrary(rootPath, baseNamespace, baseImportPath, &catalog, &allMetadata, debug)
	}

	updateGlobalCatalog(catalog)

	// 4. Generate Code
	outputDir, err := resolveOutputDir(outputFlag)
	if err != nil {
		fmt.Printf("Error resolving output directory: %v\n", err)
		fmt.Println("Tip: Use --output <path> to specify the 'nexus/generated' folder.")
		return
	}

	// Dump Local Catalog
	writeLocalCatalog(catalog, outputDir)

	if catalogOnly {
		fmt.Println("Catalog updated. Skipping code generation (--catalog-only).")
		return
	}

	fmt.Printf("Writing generated code to: %s\n", outputDir)

	if err := generator.GenerateServer(catalog, allMetadata, outputDir); err != nil {
		fmt.Printf("Error generating server: %v\n", err)
	} else {
		fmt.Println("Server code generated.")
	}

	if err := generator.GenerateSDK(catalog, outputDir); err != nil {
		fmt.Printf("Error generating SDK: %v\n", err)
	} else {
		fmt.Println("SDK code generated.")
	}

	if err := generator.GenerateTypes(catalog, outputDir); err != nil {
		fmt.Printf("Error generating Types: %v\n", err)
	} else {
		fmt.Println("Types code generated.")
	}
}

func updateGlobalCatalog(cat model.Catalog) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	globalDir := filepath.Join(home, ".nexus")
	os.MkdirAll(globalDir, 0755)

	fGlobal, err := os.Create(filepath.Join(globalDir, "catalog.json"))
	if err != nil {
		log.Fatal(err)
	}
	defer fGlobal.Close()

	encGlobal := json.NewEncoder(fGlobal)
	encGlobal.SetIndent("", "  ")
	encGlobal.Encode(cat)
	fmt.Printf("Success. Catalog updated: %s\n", filepath.Join(globalDir, "catalog.json"))
}

func writeLocalCatalog(cat model.Catalog, outputDir string) {
	f, err := os.Create(filepath.Join(outputDir, "catalog.json"))
	if err != nil {
		fmt.Printf("Warning: Could not write catalog.json: %v\n", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(cat)
	fmt.Println("Catalog saved to generated folder.")
}

func execCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}
