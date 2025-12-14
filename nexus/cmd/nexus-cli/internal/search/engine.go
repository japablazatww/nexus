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

	for _, svc := range catalog.Services {
		// Check Inputs
		for _, param := range svc.Inputs {
			if normalize(param.Name) == normalizedQuery {
				results = append(results, model.SearchResult{
					Namespace:    svc.Namespace,
					Method:       svc.Method,
					MatchedParam: param.Name,
					ParamType:    "Input",
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
				})
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
