// Command logcodemod rewrites printf-style log calls into applog calls with
// typed fields (ADR-0003 decision 9).
//
//	cd server && go run ./tools/logcodemod ./internal ./pkg ./cmd/reearth-accounts-admin
//
// Each argument maps to a field constructor through the rules table below. An
// argument that carries personal data maps to drop. A call with an argument
// the table does not know is left unchanged and printed, so a rerun after a
// rebase shows exactly which new call sites need a rule.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	applogPath = "github.com/reearth/reearth-accounts/server/pkg/applog"
	drop       = "-"
)

var logPackages = map[string]bool{
	"github.com/reearth/reearthx/log": true,
	"github.com/labstack/gommon/log":  true,
	"log":                             true,
}

// rules maps an argument's source text to a field constructor in applog, or to
// drop. A "file.go|expr" key applies only in files with that base name.
var rules = map[string]string{
	"err": "Err", "cerr": "Err", "cipErr": "Err", "jsonErr": "Err", "lockErr": "Err",
	"migrationErr": "Err", "roleErr": "Err", "sErr": "Err", "serr": "Err", "terr": "Err",
	"merr": "Err", "perr": "Err", "resp.Err": "Err",
	"evt.Failure": "ErrMessage",

	"u.ID": "UserID", "uid": "UserID", "userID": "UserID", "userId": "UserID",
	"usr.ID()": "UserID", "d.ID": "UserID", "m.UserMember.UserID": "UserID",
	"q.Me.ID": "UserID", "q.User.ID": "UserID", "m.Logout.ID": "UserID",
	"m.RemoveMyAuth.Me.ID": "UserID", "m.Signup.User.ID": "UserID",
	"m.SignupOIDC.User.ID": "UserID", "m.UpdateMe.Me.ID": "UserID",
	"m.VerifyUser.User.ID": "UserID",

	"q.Me.MyWorkspaceID": "WorkspaceID", "q.User.Workspace": "WorkspaceID",
	"u.Workspace": "WorkspaceID", "m.Logout.MyWorkspaceID": "WorkspaceID",
	"m.RemoveMyAuth.Me.MyWorkspaceID": "WorkspaceID", "m.UpdateMe.Me.MyWorkspaceID": "WorkspaceID",
	"m.VerifyUser.User.Workspace": "WorkspaceID", "in.WorkspaceID": "WorkspaceID",
	"res.Workspace.ID": "WorkspaceID", "t.ID()": "WorkspaceID", "w.ID": "WorkspaceID",
	"w.ID()": "WorkspaceID", "wid": "WorkspaceID", "workpaceId": "WorkspaceID",
	"ws.ID()": "WorkspaceID", "wsDoc.ID": "WorkspaceID",
	"wids": "WorkspaceIDs", "workpaceIds": "WorkspaceIDs",

	"admin.ID()": "AdminID", "res.NewOwner.UserID": "NewOwnerID",
	"res.DemotedUserIDs": "DemotedUserIDs", "integrationID": "IntegrationID",
	"confirmed[i].ID": "EnrollmentID", "ai.Sub": drop, "param.Sub": drop, // subjects are returned in Me.auths, a SELF field

	"roleName": "Role", "roleNames": "Roles", "param.Action": "Action",
	"result.Actions": "Actions", "allowed": "Allowed",

	"authenticator.go|path": "Path(auth0Route(%s))", // Auth0 paths embed the user ID "resp.StatusCode": "Status", "resp.Status": "Status", "status": "Status",
	"code": "Code", "resp.Message": "Reason", "len(ids)": "Count", "len(response.Errors)": "Count",
	"spanName": "Operation",

	"lock.go|name": "LockName", "lockID": "LockID", "r.hostid": "Host",
	"evt.CommandName": "Command", "evt.Duration": "DurationMS",
	"s.serviceName": "Service", "string(cfg.ServiceName)": "Service",
	"string(otelapp.OtelAccountsServiceName)": "Service",
	"s.address": "Address", "w.address": "Address", "app.go|w": "Window",
	"conf.Print()": "Config", "masked": "Config", "res.Attributes()": "Resource",
	"cfg.Endpoint": "Endpoint", "resolvedEndpoint": "Endpoint", "cfg.Insecure": "Insecure",
	"cfg.ExporterType": "Exporter", "ratio": "SamplingRatio", "cfg.SamplingRatio": "SamplingRatio",
	"cfg.BatchTimeout": "BatchTimeout", "cfg.MaxExportBatchSize": "MaxExportBatchSize",
	"cfg.MaxQueueSize": "MaxQueueSize",

	// Personal data, request content, or noise. Never logged.
	"param.Email": drop, "usr.Name()": drop, "userName": drop, "name": drop, "alias": drop,
	"FIXED_MOCK_USERNAME": drop, "debugUser": drop, "maskEmail(wsDoc.Name)": drop,
	"rc.RawQuery": drop, "rc.Variables": drop, "evt.Reply": drop, "r.f": drop,
	"authenticator.go|r": drop, "debugLog": drop,
}

var levels = map[string]string{
	"Trace": "Debug", "Debug": "Debug", "Info": "Info", "Print": "Info", "Println": "Info",
	"Warn": "Warn", "Error": "Error", "Fatal": "Fatal", "Panic": "Panic",
}

var (
	verbRe  = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)
	spaceRe = regexp.MustCompile(`\s+`)
)

type unmapped struct{ pos, reason string }

func main() {
	changed, skipped, err := run(os.Args[1:], true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("rewrote %d calls\n", changed)
	for _, s := range skipped {
		fmt.Printf("left unchanged: %s: %s\n", s.pos, s.reason)
	}
}

func run(roots []string, write bool) (int, []unmapped, error) {
	var skipped []unmapped
	changed := 0
	for _, root := range roots {
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && (info.Name() == "gen" || info.Name() == "testdata") {
				return filepath.SkipDir
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			n, s, err := rewriteFile(p, write)
			changed += n
			skipped = append(skipped, s...)
			return err
		})
		if err != nil {
			return 0, nil, err
		}
	}
	return changed, skipped, nil
}

type edit struct {
	start, end int
	text       string
}

func rewriteFile(path string, write bool) (int, []unmapped, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return 0, nil, err
	}
	logName := ""
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if logPackages[p] {
			logName = "log"
			if imp.Name != nil {
				logName = imp.Name.Name
			}
		}
	}
	if logName == "" {
		return 0, nil, nil
	}

	var edits []edit
	var skipped []unmapped
	needContext := false
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != logName {
			return true
		}
		text, bg, reason, ok := convert(fset, src, path, sel.Sel.Name, call, stack)
		pos := fset.Position(call.Pos()).String()
		if !ok {
			if reason != "" {
				skipped = append(skipped, unmapped{pos, reason})
			}
			return true
		}
		needContext = needContext || bg
		edits = append(edits, edit{fset.Position(call.Pos()).Offset, fset.Position(call.End()).Offset, text})
		return true
	})
	if len(edits) == 0 || !write {
		return len(edits), skipped, nil
	}

	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), src...)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}

	fset2 := token.NewFileSet()
	f2, err := parser.ParseFile(fset2, path, out, parser.ParseComments)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: rewritten source does not parse: %w", path, err)
	}
	astutil.AddImport(fset2, f2, applogPath)
	if needContext {
		astutil.AddImport(fset2, f2, "context")
	}
	if !uses(f2, logName) {
		for p := range logPackages {
			astutil.DeleteImport(fset2, f2, p)
		}
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset2, f2); err != nil {
		return 0, nil, err
	}
	return len(edits), skipped, os.WriteFile(path, buf.Bytes(), 0o644)
}

func uses(f *ast.File, name string) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == name {
				found = true
			}
		}
		return !found
	})
	return found
}

func srcOf(fset *token.FileSet, src []byte, n ast.Node) string {
	return string(src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
}

func convert(fset *token.FileSet, src []byte, path, method string, call *ast.CallExpr, stack []ast.Node) (string, bool, string, bool) {
	name := method
	hasCtx := false
	formatted := false
	if strings.HasSuffix(name, "fc") {
		name, hasCtx, formatted = strings.TrimSuffix(name, "fc"), true, true
	} else if strings.HasSuffix(name, "c") && levels[strings.TrimSuffix(name, "c")] != "" {
		name, hasCtx = strings.TrimSuffix(name, "c"), true
	} else if strings.HasSuffix(name, "f") && levels[strings.TrimSuffix(name, "f")] != "" {
		name, formatted = strings.TrimSuffix(name, "f"), true
	}
	lv := levels[name]
	if lv == "" {
		return "", false, "", false
	}

	args := call.Args
	ctxExpr, bg := "", false
	if hasCtx {
		ctxExpr, args = srcOf(fset, src, args[0]), args[1:]
	} else {
		ctxExpr, bg = findContext(stack)
	}
	if len(args) == 0 {
		return "", false, "no arguments", false
	}

	if !formatted && len(args) == 1 {
		if sp, ok := args[0].(*ast.CallExpr); ok {
			if s, ok := sp.Fun.(*ast.SelectorExpr); ok && srcOf(fset, src, s) == "fmt.Sprintf" {
				args, formatted = sp.Args, true
			}
		}
	}

	var msg string
	var values []ast.Expr
	if formatted {
		lit, ok := args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false, "format is not a string literal", false
		}
		format, _ := strconv.Unquote(lit.Value)
		msg = messageFromFormat(format)
		values = args[1:]
	} else {
		lit, ok := args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false, "message is not a string literal", false
		}
		m, _ := strconv.Unquote(lit.Value)
		msg = clean(m)
		for _, a := range args[1:] {
			if l, ok := a.(*ast.BasicLit); ok && l.Kind == token.STRING {
				continue // a "key" label in a non-w call; the value maps through rules
			}
			values = append(values, a)
		}
	}

	if msg == "" {
		return "", false, "empty message; add an override", false
	}

	fields := make([]string, 0, len(values))
	base := filepath.Base(path)
	for _, v := range values {
		text := srcOf(fset, src, v)
		inner := strings.TrimSuffix(strings.TrimSuffix(text, ".Error()"), ".String()")
		ctor, ok := rules[base+"|"+inner]
		if !ok {
			ctor, ok = rules[inner]
		}
		if !ok {
			return "", false, "no rule for argument " + text, false
		}
		if ctor == drop {
			continue
		}
		if strings.Contains(ctor, "%s") {
			fields = append(fields, "applog."+fmt.Sprintf(ctor, inner))
			continue
		}
		fields = append(fields, fmt.Sprintf("applog.%s(%s)", ctor, inner))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "applog.%s(%s, %s", lv, ctxExpr, strconv.Quote(msg))
	for _, fl := range fields {
		b.WriteString(", ")
		b.WriteString(fl)
	}
	b.WriteString(")")
	return b.String(), bg, "", true
}

// overrides names the message for formats whose verbs sit mid-sentence.
var overrides = map[string]string{
	"graphql: operation '%s' completed with %d errors":               "graphql: operation completed with errors",
	"graphql: operation '%s' completed successfully":                 "graphql: operation completed",
	"server started%s at http://%s\n":                                "server started",
	"OTEL sampling ratio %f is below 0; clamping to 0 (NeverSample)": "OTEL sampling ratio is below 0; clamping to 0",
	"OTEL sampling ratio %f exceeds 1; clamping to 1 (AlwaysSample)": "OTEL sampling ratio exceeds 1; clamping to 1",
	"Action %s not found in result.Actions":                          "cerbos: action not found in result",
	"RoleNames: %+v, Result Actions: %+v":                            "cerbos: check result",
	"Final permission result for user %s: %v":                        "cerbos: final permission result",
	"[admin] transfer workspace owner: role record %q is missing":    "[admin] transfer workspace owner: role record is missing",
	"WorkspaceFilter: %+v":                                           "workspace filter denied write",
	"Workspace: %+v":                                                 "workspace filter denied write",
	"rest: %d %s: %v":                                                "rest: request failed",
	"auth0: path: %s, status: %d":                                    "auth0: response",
}

var parenVerbRe = regexp.MustCompile(`\s*\([^()]*%[^()]*\)`)
var trailingKeyRe = regexp.MustCompile(`[\w.]+=$`)

func messageFromFormat(format string) string {
	if m, ok := overrides[format]; ok {
		return m
	}
	s := parenVerbRe.ReplaceAllString(format, "")
	if loc := verbRe.FindStringIndex(s); loc != nil {
		s = trailingKeyRe.ReplaceAllString(strings.TrimRight(s[:loc[0]], " "), "")
	}
	return clean(s)
}

func clean(s string) string {
	s = spaceRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " ,", ",")
	for {
		t := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), ":,=-"))
		t = strings.TrimSuffix(t, " for")
		if t == s {
			return s
		}
		s = t
	}
}

// findContext returns an expression for the nearest context in scope: a
// context.Context parameter, a local ctx, or an echo.Context's request context.
func findContext(stack []ast.Node) (string, bool) {
	for i := len(stack) - 1; i >= 0; i-- {
		var ft *ast.FuncType
		var body *ast.BlockStmt
		switch fn := stack[i].(type) {
		case *ast.FuncDecl:
			ft, body = fn.Type, fn.Body
		case *ast.FuncLit:
			ft, body = fn.Type, fn.Body
		default:
			continue
		}
		for _, p := range ft.Params.List {
			t := exprString(p.Type)
			for _, n := range p.Names {
				if t == "context.Context" {
					return n.Name, false
				}
				if t == "echo.Context" {
					return n.Name + ".Request().Context()", false
				}
			}
		}
		if declaresCtx(body, stack[len(stack)-1].Pos()) {
			return "ctx", false
		}
	}
	return "context.Background()", true
}

func declaresCtx(body *ast.BlockStmt, before token.Pos) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && as.Tok == token.DEFINE && as.Pos() < before {
			for _, l := range as.Lhs {
				if id, ok := l.(*ast.Ident); ok && id.Name == "ctx" {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func exprString(e ast.Expr) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, token.NewFileSet(), e)
	return b.String()
}
