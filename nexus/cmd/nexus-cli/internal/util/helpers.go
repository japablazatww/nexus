package util

import (
	"go/ast"
	"strings"
	"unicode"
)

func ToSnakeCase(str string) string {
	var result strings.Builder
	runes := []rune(str)
	length := len(runes)

	for i := 0; i < length; i++ {
		r := runes[i]
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

func ToPascalCase(str string) string {
	if len(str) == 0 {
		return ""
	}
	return strings.ToUpper(str[:1]) + str[1:]
}

func TypeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + TypeToString(t.X)
	case *ast.SelectorExpr:
		return TypeToString(t.X) + "." + t.Sel.Name
	default:
		return "interface{}"
	}
}
