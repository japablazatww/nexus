package generator

const ServerTemplate = `package generated

import (
	"encoding/json"
	"net/http"
	"strings"
    
	{{range $path, $alias := .Imports}}
	{{$alias}} "{{$path}}"
	{{end}}
)

func RegisterHandlers(mux *http.ServeMux) {
	{{range .Handlers}}
	mux.HandleFunc("/{{.Route}}", handle{{.FuncAlias}}_{{.FuncName}})
	{{end}}
}

{{range .Handlers}}
func handle{{.FuncAlias}}_{{.FuncName}}(w http.ResponseWriter, r *http.Request) {
	var req GenericRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Extract Parameters
	params := req.Params
	
	// 2. Call Implementation
	{{if .Outputs}}resp, err := {{else}}{{end}}wrapper{{.FuncAlias}}_{{.FuncName}}(params)
	
	// 3. Response
	w.Header().Set("Content-Type", "application/json")
	{{if .Outputs}}
	if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
	}
	json.NewEncoder(w).Encode(resp)
	{{else}}
	w.WriteHeader(http.StatusOK)
	{{end}}
}

func wrapper{{.FuncAlias}}_{{.FuncName}}(params map[string]interface{}) ({{if .Outputs}}interface{}, error{{else}}{{end}}) {
    // Inputs: {{range $i, $e := .Inputs}}{{if gt $i 0}}, {{end}}{{$e.Name}}({{$e.Type}}){{end}}
    
    {{$alias := .FuncAlias}}
    {{range .Inputs}}
    
    // Determine Type string (Primitive vs Complex)
    {{if or (eq .Type "string") (eq .Type "float64") (eq .Type "int") (eq .Type "bool")}}
    var val_{{.Name}} {{.Type}}
    {{else}}
    var val_{{.Name}} {{$alias}}.{{.Type}}
    {{end}}

    // Fuzzy Match Logic
    found_{{.Name}} := false
    target_{{.Name}} := strings.ToLower(strings.ReplaceAll("{{.Name}}", "_", ""))
    
    for k, v := range params {
        normalizedK := strings.ToLower(strings.ReplaceAll(k, "_", ""))
        if normalizedK == target_{{.Name}} {
            {{if eq .Type "string"}}
            val_{{.Name}}, _ = v.(string)
            {{else if eq .Type "float64"}}
            val_{{.Name}}, _ = v.(float64)
            {{else if eq .Type "int"}}
             // JSON numbers are float64
             if fVal, ok := v.(float64); ok {
                 val_{{.Name}} = int(fVal)
             }
            {{else if eq .Type "bool"}}
            val_{{.Name}}, _ = v.(bool)
            {{else}}
            // Complex Type: Convert map -> json -> struct
            jsonBody, _ := json.Marshal(v)
            json.Unmarshal(jsonBody, &val_{{.Name}})
            {{end}}
            found_{{.Name}} = true
            break
        }
    }
    
    if !found_{{.Name}} {
       // Optional: Log or Error if required param missing?
    }
    {{end}}

    // Call
    {{if .HasError}}
        {{if eq .NumReturns 2}}
            // Expected: (val, error)
            ret0, err := {{$alias}}.{{.FuncName}}({{range $i, $e := .Inputs}}{{if gt $i 0}}, {{end}}val_{{$e.Name}}{{end}})
            if err != nil {
                return nil, err
            }
            return ret0, nil
        {{else}}
             // Expected: (error) ONLY? or (val1, val2, error) - simplifying to (error)
             err := {{$alias}}.{{.FuncName}}({{range $i, $e := .Inputs}}{{if gt $i 0}}, {{end}}val_{{$e.Name}}{{end}})
             return nil, err
        {{end}}
    {{else}}
        // No error returned
        {{if gt .NumReturns 0}}
            // Expected: (val)
            ret0 := {{$alias}}.{{.FuncName}}({{range $i, $e := .Inputs}}{{if gt $i 0}}, {{end}}val_{{$e.Name}}{{end}})
            return ret0, nil
        {{else}}
            // Expected: void
            {{$alias}}.{{.FuncName}}({{range $i, $e := .Inputs}}{{if gt $i 0}}, {{end}}val_{{$e.Name}}{{end}})
            return nil, nil
        {{end}}
    {{end}}
}
{{end}}
`

const SDKTemplate = `package generated

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)



type Transport interface {
	Call(method string, req GenericRequest) (interface{}, error)
}

type httpTransport struct {
	BaseURL string
	Client  *http.Client
}

func (t *httpTransport) Call(method string, req GenericRequest) (interface{}, error) {
	body, _ := json.Marshal(req)
	resp, err := t.Client.Post(t.BaseURL + "/" + method, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}
	
	var result interface{}
	// Decode logic... for now just simple
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Structs ---

{{range $struct := .Structs}}
type {{$struct.Name}} struct {
	transport Transport
	{{range .Fields}}
	{{.}}
	{{end}}
}

{{range .Methods}}
func (c *{{$struct.Name}}) {{.Method}}(req GenericRequest) (interface{}, error) {
	return c.transport.Call("{{.Namespace}}.{{.Method}}", req)
}
{{end}}
{{end}}

func NewClient(baseURL string) *Client {
	t := &httpTransport{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
	c := &Client{transport: t}
	
	// Dynamic Init
{{.InitCode}}

	return c
}
`

const TypesTemplate = `package generated

// --- Shared Types ---

type GenericRequest struct {
	Params map[string]interface{} ` + "`" + `json:"params"` + "`" + `
}

{{range .Structs}}
type {{.Name}} struct {
{{range .Fields}}
    {{.Name}} {{.Type}} ` + "`" + `json:"{{.JSONTag}}"` + "`" + `
{{end}}
}
{{end}}
`
