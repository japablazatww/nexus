package search

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/model"
)

func ResolveDefaultCatalog() string {
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".nexus", "catalog.json")
	}
	return "catalog.json"
}

func SearchByParam(catalog model.Catalog, query string) []model.SearchResult {
	var results []model.SearchResult
	normalizedQuery := normalize(query)

	// 1. Build a map of "StructName -> Services that use it" for fast lookup
	// This handles standard usage where a service uses a struct as Input or Output.
	// We want to find "Who uses LoanRequest?" -> Service A
	structUsage := make(map[string][]model.ServiceEntry)

	for _, svc := range catalog.Services {
		// Check Inputs
		for _, inp := range svc.Inputs {
			// Naive check: does the type name contain the struct name?
			// Type could be "*LoanRequest", "[]LoanRequest", "LoanRequest"
			// We will just store the service for every param type involved.
			cleanType := strings.TrimLeft(inp.Type, "[]*")
			structUsage[cleanType] = append(structUsage[cleanType], svc)
		}
		// Check Outputs
		for _, out := range svc.Outputs {
			cleanType := strings.TrimLeft(out.Type, "[]*")
			structUsage[cleanType] = append(structUsage[cleanType], svc)
		}
	}

	// 2. Search Services Inputs/Outputs (Direct Param Match)
	for _, svc := range catalog.Services {
		// Check Inputs
		for _, param := range svc.Inputs {
			if normalize(param.Name) == normalizedQuery {
				results = append(results, model.SearchResult{
					Namespace:    svc.Namespace,
					Method:       svc.Method,
					MatchedParam: param.Name,
					ParamType:    "Input",
					Description:  svc.Description,
				})
			}
		}
		// Check Outputs
		for _, param := range svc.Outputs {
			if normalize(param.Name) == normalizedQuery {
				results = append(results, model.SearchResult{
					Namespace:    svc.Namespace,
					Method:       svc.Method,
					MatchedParam: param.Name,
					ParamType:    "Output",
					Description:  svc.Description,
				})
			}
		}
	}

	// 3. Search Structs (Struct Name Match OR Field Match)
	for _, s := range catalog.Structs {
		structMatch := normalize(s.Name) == normalizedQuery
		fieldMatch := ""

		if !structMatch {
			for _, f := range s.Fields {
				if normalize(f.Name) == normalizedQuery {
					fieldMatch = f.Name
					break // Found a matching field
				}
			}
		}

		if structMatch || fieldMatch != "" {
			// Find services using this struct
			servicesUsing, ok := structUsage[s.Name]
			if ok {
				for _, svc := range servicesUsing {
					res := model.SearchResult{
						Namespace:   svc.Namespace,
						Method:      svc.Method,
						Description: svc.Description,
						StructName:  s.Name,
					}
					if structMatch {
						res.MatchedParam = s.Name
						res.ParamType = "Struct"
					} else {
						res.MatchedParam = s.Name + "." + fieldMatch
						res.ParamType = "Struct Field"
						res.FieldName = fieldMatch
					}
					results = append(results, res)
				}
			}
		}
	}

	return results
}

func normalize(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "_", ""))
}

func LoadCatalog(path string) (model.Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Catalog{}, err
	}
	var catalog model.Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return model.Catalog{}, fmt.Errorf("invalid JSON content: %w", err)
	}
	return catalog, nil
}
