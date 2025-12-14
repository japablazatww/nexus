package generator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/model"
	"github.com/japablazatww/nexus/nexus/cmd/nexus-cli/internal/util"
)

func GenerateServer(catalog model.Catalog, metadata []model.FunctionMetadata, outputDir string) error {
	imports := make(map[string]string) // path -> alias

	type HandlerData struct {
		Route      string
		FuncAlias  string
		FuncName   string
		Inputs     []model.ParamMetadata
		Outputs    []model.ParamMetadata
		HasError   bool
		NumReturns int
	}

	handlers := []HandlerData{}

	for _, svc := range catalog.Services {
		importPath := svc.ImportPath

		alias := strings.ReplaceAll(svc.Namespace, ".", "_")
		alias = strings.ReplaceAll(alias, "-", "_")

		imports[importPath] = alias

		hasError := false
		if len(svc.Outputs) > 0 {
			last := svc.Outputs[len(svc.Outputs)-1]
			if last.Type == "error" {
				hasError = true
			}
		}

		handlers = append(handlers, HandlerData{
			Route:      svc.Namespace + "." + svc.Method,
			FuncAlias:  alias,
			FuncName:   svc.Method,
			Inputs:     svc.Inputs,
			Outputs:    svc.Outputs,
			HasError:   hasError,
			NumReturns: len(svc.Outputs),
		})
	}

	f, err := os.Create(filepath.Join(outputDir, "server_gen.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	return executeTemplate(f, ServerTemplate, map[string]interface{}{
		"Imports":  imports,
		"Handlers": handlers,
	})
}

func GenerateSDK(catalog model.Catalog, outputDir string) error {
	// Tree structure
	type Node struct {
		Name     string // e.g. "System"
		Children map[string]*Node
		Methods  []model.ServiceEntry
	}

	root := &Node{Name: "Client", Children: make(map[string]*Node)}

	for _, svc := range catalog.Services {
		// Split namespace: libreria-a.transfers.national
		parts := strings.Split(svc.Namespace, ".")

		current := root
		for _, p := range parts {
			// Normalize PascalCase for Struct fields
			p = util.ToPascalCase(strings.ReplaceAll(p, "-", "")) // libreria-a -> LibreriaA

			if _, exists := current.Children[p]; !exists {
				current.Children[p] = &Node{Name: p, Children: make(map[string]*Node)}
			}
			current = current.Children[p]
		}
		current.Methods = append(current.Methods, svc)
	}

	// Flatten tree to generate structs
	type StructDef struct {
		Name    string
		Fields  []string // "System *LibreriaASystemClient"
		Methods []model.ServiceEntry
	}

	var structs []StructDef

	// BFS or DFS to traverse and build structs
	var traverse func(n *Node, prefix string) string // returns TypeName
	traverse = func(n *Node, prefix string) string {
		var typeName string
		// Special case for Root
		if n == root {
			typeName = "Client"
		} else {
			typeName = prefix + n.Name + "Client"
		}

		myStruct := StructDef{Name: typeName}

		// Compute next prefix for children
		var nextPrefix string
		if n == root {
			nextPrefix = ""
		} else {
			nextPrefix = prefix + n.Name
		}

		for childName, childNode := range n.Children {
			childType := traverse(childNode, nextPrefix)
			myStruct.Fields = append(myStruct.Fields, fmt.Sprintf("%s *%s", childName, childType))
		}

		myStruct.Methods = n.Methods
		structs = append(structs, myStruct)
		return typeName
	}

	traverse(root, "")

	// --- Dynamic Init Code Generation ---
	var initLines []string
	var genInit func(n *Node, accessPath string, typePrefix string)
	genInit = func(n *Node, accessPath string, typePrefix string) {
		for childName, childNode := range n.Children {
			childAccess := accessPath + "." + childName
			childType := typePrefix + childName + "Client"

			initLines = append(initLines, fmt.Sprintf("\t%s = &%s{transport: t}", childAccess, childType))

			genInit(childNode, childAccess, typePrefix+childName)
		}
	}

	genInit(root, "c", "")

	initCode := strings.Join(initLines, "\n")

	f, err := os.Create(filepath.Join(outputDir, "sdk_gen.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	return executeSDKTemplate(f, SDKTemplate, structs, initCode)
}

func GenerateTypes(catalog model.Catalog, outputDir string) error {
	f, err := os.Create(filepath.Join(outputDir, "types_gen.go"))
	if err != nil {
		return err
	}
	defer f.Close()
	return executeTemplate(f, TypesTemplate, catalog)
}

func executeTemplate(w io.Writer, tmplStr string, data interface{}) error {
	t, err := template.New("gen").Parse(tmplStr)
	if err != nil {
		return err
	}
	return t.Execute(w, data)
}

func executeSDKTemplate(w io.Writer, tmplStr string, structs interface{}, initCode string) error {
	t, err := template.New("sdk").Parse(tmplStr)
	if err != nil {
		return err
	}

	data := struct {
		Structs  interface{}
		InitCode string
	}{
		Structs:  structs,
		InitCode: initCode,
	}

	return t.Execute(w, data)
}
