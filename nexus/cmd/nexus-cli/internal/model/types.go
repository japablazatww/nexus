package model

// --- Structs ---

type LibConfig struct {
	HasNestedDomains bool     `json:"hasNestedDomains"`
	Domains          []string `json:"domains"`
	IsDomain         bool     `json:"isDomain"`
}

type FunctionMetadata struct {
	Name           string
	Params         []Param
	Returns        []string
	RequestStruct  string
	ResponseStruct string
	Comment        string
}

type Param struct {
	Name      string
	Type      string
	JSONTag   string
	FieldName string // PascalCase for struct
}

type StructMetadata struct {
	Name      string
	JsonName  string // Snake case of struct name for potential usage
	Fields    []StructField
	Namespace string
}

type StructField struct {
	Name    string
	Type    string
	JSONTag string
}

type Catalog struct {
	Services []ServiceEntry   `json:"services"`
	Structs  []StructMetadata `json:"structs"`
}

type ServiceEntry struct {
	Namespace   string          `json:"namespace"`
	Method      string          `json:"method"`
	ImportPath  string          `json:"import_path"`
	Description string          `json:"description"`
	Inputs      []ParamMetadata `json:"inputs"`
	Outputs     []ParamMetadata `json:"outputs"`
}

type ParamMetadata struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type SearchResult struct {
	Namespace    string
	Method       string
	MatchedParam string
	ParamType    string // "Input", "Output", "Struct", "Struct Field"
	Description  string // Service description
	StructName   string // Name of the struct if matched (optional)
	FieldName    string // Name of the field if matched (optional)
}
