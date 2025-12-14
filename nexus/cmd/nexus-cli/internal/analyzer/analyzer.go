package analyzer

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/model"
	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/util"
)

func CrawlLibrary(currentPath string, currentNamespace string, currentImportPath string, catalog *model.Catalog, allMetadata *[]model.FunctionMetadata, debug bool) {
	if debug {
		fmt.Printf("DEBUG: Crawling %s (NS: %s) [Import: %s]\n", currentPath, currentNamespace, currentImportPath)
	}

	// 1. Read lib_config.json
	configFile := filepath.Join(currentPath, "lib_config.json")
	configData, err := os.ReadFile(configFile)
	if err != nil {
		if debug {
			fmt.Printf("DEBUG: No lib_config.json in %s\n", currentPath)
		}
		return
	}

	var config model.LibConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		if debug {
			fmt.Printf("DEBUG: Invalid lib_config.json in %s: %v\n", currentPath, err)
		}
		return
	}

	// 2. If it is a domain with functions, parse them
	if config.IsDomain {
		if debug {
			fmt.Printf("DEBUG: Found Domain at %s. Parsing functions...\n", currentNamespace)
		}
		meta, entries, structs := ParseLibrary(currentPath, currentNamespace, currentImportPath, debug)
		catalog.Services = append(catalog.Services, entries...)
		catalog.Structs = append(catalog.Structs, structs...)
		*allMetadata = append(*allMetadata, meta...)
	}

	// 3. If it has nested domains, recurse
	if config.HasNestedDomains {
		for _, domain := range config.Domains {
			subPath := filepath.Join(currentPath, domain)
			// Construct nested namespace: libreria-a.transfers.national
			subNamespace := fmt.Sprintf("%s.%s", currentNamespace, domain)
			// Construct import path
			subImportPath := fmt.Sprintf("%s/%s", currentImportPath, domain)
			CrawlLibrary(subPath, subNamespace, subImportPath, catalog, allMetadata, debug)
		}
	}
}

func ParseLibrary(path string, namespace string, importPath string, debug bool) ([]model.FunctionMetadata, []model.ServiceEntry, []model.StructMetadata) {
	fset := token.NewFileSet()
	// Parse only .go files in this directory
	pkgs, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
	if err != nil {
		log.Printf("Warning: error parsing %s: %v", path, err)
		return nil, nil, nil
	}

	var metadata []model.FunctionMetadata
	var entries []model.ServiceEntry
	var structs []model.StructMetadata

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				// 1. Structs
				if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							if structType, ok := typeSpec.Type.(*ast.StructType); ok {
								if !typeSpec.Name.IsExported() {
									continue
								}
								structName := typeSpec.Name.Name
								var fields []model.StructField
								for _, field := range structType.Fields.List {
									fType := util.TypeToString(field.Type)
									// Parse Tag
									tag := ""
									if field.Tag != nil {
										tag = strings.ReplaceAll(field.Tag.Value, "`", "")
										tag = strings.ReplaceAll(tag, "json:\"", "")
										tag = strings.ReplaceAll(tag, "\"", "")
									}
									for _, name := range field.Names {
										fields = append(fields, model.StructField{
											Name:    name.Name,
											Type:    fType,
											JSONTag: tag,
										})
									}
								}
								structs = append(structs, model.StructMetadata{
									Name:      structName,
									JsonName:  util.ToSnakeCase(structName),
									Fields:    fields,
									Namespace: namespace,
								})
							}
						}
					}
				}

				// 2. Functions
				if fn, ok := decl.(*ast.FuncDecl); ok {
					if !fn.Name.IsExported() {
						continue
					}
					// Check convention: Files containing functions usually named 'functions.go'
					// But we parse all for now.

					fname := fn.Name.Name

					// Inputs
					inputs := []model.ParamMetadata{}
					params := []model.Param{}
					for _, field := range fn.Type.Params.List {
						typeExpr := util.TypeToString(field.Type)
						for _, name := range field.Names {
							pName := name.Name
							params = append(params, model.Param{
								Name:      pName,
								Type:      typeExpr,
								JSONTag:   util.ToSnakeCase(pName),
								FieldName: util.ToPascalCase(pName),
							})
							inputs = append(inputs, model.ParamMetadata{
								Name: util.ToSnakeCase(pName),
								Type: typeExpr,
							})
						}
					}

					// Outputs
					returns := []string{}
					outputs := []model.ParamMetadata{}
					if fn.Type.Results != nil {
						for i, field := range fn.Type.Results.List {
							typeExpr := util.TypeToString(field.Type)
							name := ""
							if len(field.Names) > 0 {
								for _, n := range field.Names {
									name = n.Name
									outputs = append(outputs, model.ParamMetadata{Name: name, Type: typeExpr})
								}
							} else {
								name = fmt.Sprintf("result_%d", i)
								outputs = append(outputs, model.ParamMetadata{Name: name, Type: typeExpr})
							}
							returns = append(returns, typeExpr)
						}
					}

					meta := model.FunctionMetadata{
						Name:          fname,
						Params:        params,
						Returns:       returns,
						RequestStruct: fname + "Request",
						Comment:       fn.Doc.Text(),
					}
					metadata = append(metadata, meta)

					entries = append(entries, model.ServiceEntry{
						Namespace:   namespace, // Namespace is passed from crawler now
						Method:      fname,
						ImportPath:  importPath,
						Description: strings.TrimSpace(fn.Doc.Text()),
						Inputs:      inputs,
						Outputs:     outputs,
					})
				}
			}
		}
	}
	return metadata, entries, structs
}

func EnsureLibraryInstalled(widthDir string, pkg string, version string, debug bool) error {
	// usage: go get pkg@version
	target := fmt.Sprintf("%s@%s", pkg, version)
	cmd := exec.Command("go", "get", target)
	cmd.Dir = widthDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running go get: %s\nOutput: %s", err, string(output))
	}
	if debug {
		fmt.Printf("\nDEBUG: go get output:\n%s\n", string(output))
	}

	return nil
}

func ResolvePackagePath(withDir string, pkg string, debug bool) (string, error) {
	// Use -m to resolve the Module Root, as the root might not be a package anymore (no .go files)
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", pkg)
	cmd.Dir = withDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		if debug {
			fmt.Printf("DEBUG: go list error output:\n%s\n", string(output))
		}
		return "", fmt.Errorf("go list failed: %v", err)
	}
	path := strings.TrimSpace(string(output))
	if debug {
		fmt.Printf("DEBUG: Raw path bytes: %x\n", path)
	}
	return path, nil
}
