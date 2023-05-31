package newt

import (
	"encoding/json"
	"fmt"
	"os" // DEBUG
	"path"
	"strings"
)

const (
	StartVar = "{{"
	EndVar   = "}}"
)

type EvalType func(string, string) (string, bool)

// RouteDSL holds the attributes need to decode
// a RouteDSL expression, match and decode against path values.
type RouteDSL struct {
	Src  string   `json:"src"`
	Dirs []string `json:"dirs,omitempty"`
	Base string   `json:"base,omitempty"`
	Ext  string   `json:"ext,omitempty"`
	// VarToType maps the variable name to a var defn
	VarToType map[string]string `json:"var_to_types,omitempty"`
	// Types maps type implementation description
	Types map[string]string `json:"-"`
	// Type name to function to Eval function (validates a variable's
	// value and extracts a value)
	TypeFn map[string]EvalType `json:"-"`
}

func (rdsl *RouteDSL) String() string {
	src, _ := json.MarshalIndent(rdsl, "", "    ")
	return string(src)
}

// varDefn evaluates a variable expression returning a var name,
// type expression.
func varDefn(src string) (string, string, error) {
	if !(strings.HasPrefix(src, StartVar) && strings.HasSuffix(src, EndVar)) {
		return "", "", fmt.Errorf("missing opening or closing curly brace delimiters")
	}
	expr := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(src, StartVar), EndVar))
	//fmt.Fprintf(os.Stderr, "DEBUG src -> %q -> expr -> %q\n", src, expr)
	if expr == "" {
		return "", "", fmt.Errorf("missing varname and type info")
	}
	// Check if we both have varname and type expression
	if !strings.Contains(expr, " ") {
		return "", "", fmt.Errorf("Invalid declaration %q missing type info", src)
	}
	// Split varname from type expression
	parts := strings.SplitN(expr, " ", 2)
	vName, tExpr := parts[0], parts[1]
	if vName == "" {
		return "", "", fmt.Errorf("missing variable name")
	}
	if tExpr == "" {
		return vName, "", fmt.Errorf("missing type expression for var %q", vName)
	}
	return vName, tExpr, nil
}

// NewRouteDSL takes a RouteDSL expression and returns a
// RouteDSLExpresion structure and error value.
func NewRouteDSL(src string) (*RouteDSL, error) {
	rdsl := new(RouteDSL)
	rdsl.Src = src
	dir, base := path.Split(src)
	dirs := strings.Split(strings.TrimSuffix(strings.TrimPrefix(dir, "/"), "/"), "/")
	rdsl.Dirs = []string{}
	// We only evalaute the extension if here are two variables defined for the last element of path.
	if strings.Count(base, StartVar) == 2 {
		//fmt.Fprintf(os.Stderr, "DEBUG have a base/ext definitions -> %q\n", base)
		parts := strings.SplitN(base, EndVar, 2)
		rdsl.Base = parts[0] + EndVar
		rdsl.Ext = parts[1]
	} else {
		rdsl.Base = base
		rdsl.Ext = ""
	}
	rdsl.VarToType = map[string]string{}

	for i, elem := range dirs {
		if strings.HasPrefix(elem, StartVar) && strings.HasSuffix(elem, EndVar) {
			varName, typeExpr, err := varDefn(elem)
			if err == nil {
				rdsl.VarToType[varName] = typeExpr
			} else {
				return nil, fmt.Errorf("(%d) %q -> %s", i, elem, err)
			}
			rdsl.Dirs = append(rdsl.Dirs, fmt.Sprintf(StartVar+"%s"+EndVar, varName))
		} else {
			rdsl.Dirs = append(rdsl.Dirs, elem)
		}
	}
	if strings.HasPrefix(rdsl.Base, StartVar) && strings.HasSuffix(rdsl.Base, EndVar) {
		varName, typeExpr, err := varDefn(rdsl.Base)
		if err == nil {
			rdsl.VarToType[varName] = typeExpr
		} else {
			return nil, fmt.Errorf("(basename) %q -> %s", rdsl.Base, err)
		}
		rdsl.Base = fmt.Sprintf(StartVar+"%s"+EndVar, varName)
	}
	if rdsl.Ext != "" {
		if strings.HasPrefix(rdsl.Ext, StartVar) && strings.HasSuffix(rdsl.Ext, EndVar) {
			varName, typeExpr, err := varDefn(rdsl.Ext)
			if err == nil {
				rdsl.VarToType[varName] = typeExpr
			} else {
				return nil, fmt.Errorf("(extname) %q -> %s", rdsl.Ext, err)
			}
			rdsl.Ext = fmt.Sprintf(StartVar+"%s"+EndVar, varName)
		}
	}
	// Finally include all the types defined route_dsl_types.go
	rdsl.TypeFn = RouteTypes
	return rdsl, nil
}

// RegisterType maps a type name to a a RouteDSLType interface.
// RouteDSLType interface must defined EvalType.
func (rdsl *RouteDSL) RegisterType(tName string, t RouteDSLType) error {
	if _, ok := rdsl.TypeFn[tName]; ok {
		return fmt.Errorf("%q previously registered", tName)
	}
	rdsl.TypeFn[tName] = t.EvalType
	return nil
}

func varName(src string) string {
	return strings.TrimSuffix(strings.TrimPrefix(src, StartVar), EndVar)
}

// evalElement takes compares a element against a value (from the path value)
// returns a variable name, interface value and bool indicating a successful match
func (rdsl *RouteDSL) evalElement(elem string, src string) (string, string, bool) {
	// Check if we workingwith a literal element or a variable defn.
	if strings.HasPrefix(elem, StartVar) {
		// handle variable path element
		vName := varName(elem)
		tExpr, ok := rdsl.VarToType[vName]
		if !ok {
			return "", "", false
		}
		fn, ok := rdsl.TypeFn[tExpr]
		if !ok {
			return vName, "", false
		}
		// Now check the type of dDir against the type expression
		val, ok := fn(tExpr, src)
		if !ok {
			// Something went wrong, path does not match.
			return "", "", false
		}
		return vName, val, true
	}
	// handle literal path element
	fmt.Fprintf(os.Stderr, "DEBUG Comparing a static element %q -> %q? %d\n", elem, src, strings.Compare(elem, src))
	return "", "", (strings.Compare(elem, src) == 0)
}

// Eval takes a path value and compares it with a Path expression.
// It returns a status boolean, map of variable names to values.
func (rdsl *RouteDSL) Eval(pathValue string) (map[string]string, bool) {
	dir, base := path.Split(pathValue)
	pDirs := strings.Split(strings.TrimSuffix(strings.TrimPrefix(dir, "/"), "/"), "/")
	pExt := path.Ext(base)
	pBase := strings.TrimSuffix(base, pExt)
	/*
	*/
		if pExt != "" { // DEBUG
			fmt.Fprintf(os.Stderr, "DEBUG pathValue -> %q\n", pathValue)
			fmt.Fprintf(os.Stderr, "DEBUG pDirs -> %+v\n", pDirs)
			fmt.Fprintf(os.Stderr, "DEBUG pBase -> %q\n", pBase)
			fmt.Fprintf(os.Stderr, "DEBUG pExt -> %q\n", pExt)
		} //DEBUG
	if rdsl.Ext == "" {
		pExt = ""
		pBase = base
	}
	if len(pDirs) != len(rdsl.Dirs) {
		return nil, false
	}
	m := map[string]string{}
	for i, elem := range rdsl.Dirs {
		vName, val, ok := rdsl.evalElement(elem, pDirs[i])
		fmt.Fprintf(os.Stderr, "DEBUG (dir) vName -> %q, val -> %q, ok -> %t\n", vName, val, ok)
		if !ok {
			return nil, false
		}
		// Check if we need to store the variable
		if vName != "" {
			m[vName] = val
		}
	}
	// Match the extension, if it contains a
	if rdsl.Ext != "" {
		fmt.Fprintf(os.Stderr, "DEBUG checking rdsl.Ext %q, pExt %q\n", rdsl.Ext, pExt)
		if vName, val, ok := rdsl.evalElement(rdsl.Ext, pExt); ok {
			fmt.Fprintf(os.Stderr, "DEBUG (ext) vName -> %q, val -> %q, ok -> %t\n", vName, val, ok)
			// Check if we need to store the variable
			if vName != "" {
				m[vName] = val
			}
		}
	}
	// Match Basename
	fmt.Fprintf(os.Stderr, "DEBUG checking rdsl.Base %q, pBase %q\n", rdsl.Base, pBase)
	vName, val, ok := rdsl.evalElement(rdsl.Base, pBase)
	if ok {
		fmt.Fprintf(os.Stderr, "DEBUG (basename) vName -> %q, val -> %q, ok -> %t\n", vName, val, ok)
		// Check if we need to store the variable
		if vName != "" {
			m[vName] = val
		}
	}
	fmt.Fprintf(os.Stderr, "DEBUG final pathValue %q -> m %+v, ok %t\n", pathValue, m, ok)
	return m, ok
}

// Resolve takes a map of varnames and values and replaces any
// occurrences found in src string resulting to a new string..
func (rdsl *RouteDSL) Resolve(m map[string]string, src string) string {
	//fmt.Fprintf(os.Stderr, "DEBUG src -> %q, m -> %+v\n", src, m)
	res := src[0:]
	for k, v := range m {
		k = StartVar + k + EndVar
		//fmt.Fprintf(os.Stderr, "DEBUG k -> %q, v -> %q\n", k, v)
		if strings.Contains(res, k) {
			res = strings.ReplaceAll(res, k, v)
		}
	}
	fmt.Fprintf(os.Stderr, "DEBUG RouteDSL.Resolve -> %q\n", res)
	return res
}