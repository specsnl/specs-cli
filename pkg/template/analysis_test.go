package template_test

import (
	"os"
	"path/filepath"
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

// buildAnalysisTemplate creates a minimal template root for analysis tests.
// yaml is the project.yaml content, files maps template/-relative paths to content.
func buildAnalysisTemplate(t *testing.T, yaml string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("writing project.yaml: %v", err)
	}
	templateDir := filepath.Join(root, "template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("creating template dir: %v", err)
	}
	for relPath, content := range files {
		abs := filepath.Join(templateDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatalf("creating parent dir for %s: %v", relPath, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
			t.Fatalf("writing %s: %v", relPath, err)
		}
	}
	return root
}

func analyzeTemplate(t *testing.T, root string) *pkgtemplate.Template {
	t.Helper()
	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return tmpl
}

func TestAnalysis_Unconditional(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"Name: \"\"\n",
		map[string]string{"file.txt": "[[.Name]]"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals
	if _, ok := conds["Name"]; ok {
		t.Error("Name should not be in conditionals (it is used unconditionally)")
	}
}

func TestAnalysis_SimpleGate(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"UseDB: false\nDbName: \"\"\n",
		map[string]string{"file.txt": "[[if .UseDB]]DB=[[.DbName]][[end]]"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals

	// UseDB is a gate variable — always needed.
	if _, ok := conds["UseDB"]; ok {
		t.Error("UseDB should not be in conditionals (it is the gate variable)")
	}

	// DbName should be conditional on UseDB being true.
	cond, ok := conds["DbName"]
	if !ok {
		t.Fatal("DbName should be in conditionals")
	}
	if !cond.Eval(map[string]any{"UseDB": true}) {
		t.Error("DbName condition should be satisfied when UseDB=true")
	}
	if cond.Eval(map[string]any{"UseDB": false}) {
		t.Error("DbName condition should not be satisfied when UseDB=false")
	}
}

func TestAnalysis_ElseBranch(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"UseDB: false\nNoDbMsg: \"\"\n",
		map[string]string{"file.txt": "[[if .UseDB]]db[[else]][[.NoDbMsg]][[end]]"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals

	cond, ok := conds["NoDbMsg"]
	if !ok {
		t.Fatal("NoDbMsg should be in conditionals (else branch)")
	}
	// Else branch: condition is NOT UseDB.
	if cond.Eval(map[string]any{"UseDB": false}) != true {
		t.Error("NoDbMsg condition should be satisfied when UseDB=false")
	}
	if cond.Eval(map[string]any{"UseDB": true}) != false {
		t.Error("NoDbMsg condition should not be satisfied when UseDB=true")
	}
}

func TestAnalysis_Not(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"UseDB: false\nFallback: \"\"\n",
		map[string]string{"file.txt": "[[if not .UseDB]][[.Fallback]][[end]]"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals

	cond, ok := conds["Fallback"]
	if !ok {
		t.Fatal("Fallback should be in conditionals")
	}
	if !cond.Eval(map[string]any{"UseDB": false}) {
		t.Error("Fallback condition should be satisfied when UseDB=false")
	}
	if cond.Eval(map[string]any{"UseDB": true}) {
		t.Error("Fallback condition should not be satisfied when UseDB=true")
	}
}

func TestAnalysis_Eq(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"DbType: \"\"\nPgPort: \"\"\n",
		map[string]string{"file.txt": `[[if eq .DbType "pg"]]port=[[.PgPort]][[end]]`},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals

	cond, ok := conds["PgPort"]
	if !ok {
		t.Fatal("PgPort should be in conditionals")
	}
	if !cond.Eval(map[string]any{"DbType": "pg"}) {
		t.Error("PgPort condition should be satisfied when DbType=pg")
	}
	if cond.Eval(map[string]any{"DbType": "mysql"}) {
		t.Error("PgPort condition should not be satisfied when DbType=mysql")
	}
}

func TestAnalysis_And(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"UseDB: false\nUseSSL: false\nCert: \"\"\n",
		map[string]string{"file.txt": "[[if and .UseDB .UseSSL]]cert=[[.Cert]][[end]]"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals

	cond, ok := conds["Cert"]
	if !ok {
		t.Fatal("Cert should be in conditionals")
	}
	if !cond.Eval(map[string]any{"UseDB": true, "UseSSL": true}) {
		t.Error("Cert condition should be satisfied when UseDB=true AND UseSSL=true")
	}
	if cond.Eval(map[string]any{"UseDB": true, "UseSSL": false}) {
		t.Error("Cert condition should not be satisfied when UseSSL=false")
	}
	if cond.Eval(map[string]any{"UseDB": false, "UseSSL": true}) {
		t.Error("Cert condition should not be satisfied when UseDB=false")
	}
}

func TestAnalysis_Nested(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"UseDB: false\nDbType: \"\"\nPgCfg: \"\"\n",
		map[string]string{"file.txt": `[[if .UseDB]][[if eq .DbType "pg"]]cfg=[[.PgCfg]][[end]][[end]]`},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals

	cond, ok := conds["PgCfg"]
	if !ok {
		t.Fatal("PgCfg should be in conditionals (nested if)")
	}
	if !cond.Eval(map[string]any{"UseDB": true, "DbType": "pg"}) {
		t.Error("PgCfg condition should be satisfied when UseDB=true and DbType=pg")
	}
	if cond.Eval(map[string]any{"UseDB": false, "DbType": "pg"}) {
		t.Error("PgCfg condition should not be satisfied when UseDB=false")
	}
	if cond.Eval(map[string]any{"UseDB": true, "DbType": "mysql"}) {
		t.Error("PgCfg condition should not be satisfied when DbType!=pg")
	}
}

func TestAnalysis_BothBranches_AlwaysNeeded(t *testing.T) {
	// Name used both inside and outside the if block — always needed.
	root := buildAnalysisTemplate(t,
		"UseDB: false\nName: \"\"\n",
		map[string]string{"file.txt": "[[.Name]]\n[[if .UseDB]]also: [[.Name]][[end]]"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals
	if _, ok := conds["Name"]; ok {
		t.Error("Name should not be conditional (referenced both inside and outside if)")
	}
}

func TestAnalysis_UnknownFn_FallsBackToAlways(t *testing.T) {
	// myFunc is not recognised by the analyser — Y falls back to always-needed.
	root := buildAnalysisTemplate(t,
		"X: false\nY: \"\"\n",
		map[string]string{"file.txt": "[[if myFunc .X]][[.Y]][[end]]"},
	)
	// Build with a FuncMap that includes myFunc so the template parses.
	// Since Get uses the real FuncMap from FuncMap(), myFunc is not in it,
	// so the template file will fail to parse — the analyser treats it as always-needed.
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals
	if _, ok := conds["Y"]; ok {
		t.Error("Y should not be in conditionals when condition function is unrecognised")
	}
}

func TestAnalysis_MultiFile_ConflictBecomesAlways(t *testing.T) {
	// DbName: unconditional in file1.txt, conditional in file2.txt.
	// Result: always needed (conflict → absent from map).
	root := buildAnalysisTemplate(t,
		"UseDB: false\nDbName: \"\"\n",
		map[string]string{
			"file1.txt": "name=[[.DbName]]",
			"file2.txt": "[[if .UseDB]][[.DbName]][[end]]",
		},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals
	if _, ok := conds["DbName"]; ok {
		t.Error("DbName should not be conditional when used unconditionally in another file")
	}
}

func TestAnalysis_Filename_GateVarAlwaysNeeded(t *testing.T) {
	// UseDB appears as the gate in a filename — it must be always prompted.
	root := buildAnalysisTemplate(t,
		"UseDB: false\n",
		map[string]string{"[[if .UseDB]]db.env[[end]]": "content"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals
	if _, ok := conds["UseDB"]; ok {
		t.Error("UseDB should not be conditional when it is used as a gate in a filename")
	}
}

func TestAnalysis_Or(t *testing.T) {
	root := buildAnalysisTemplate(t,
		"UseA: false\nUseB: false\nSecret: \"\"\n",
		map[string]string{"file.txt": "[[if or .UseA .UseB]]s=[[.Secret]][[end]]"},
	)
	tmpl := analyzeTemplate(t, root)
	conds := tmpl.Conditionals

	cond, ok := conds["Secret"]
	if !ok {
		t.Fatal("Secret should be in conditionals")
	}
	if !cond.Eval(map[string]any{"UseA": true, "UseB": false}) {
		t.Error("Secret condition should be satisfied when UseA=true")
	}
	if !cond.Eval(map[string]any{"UseA": false, "UseB": true}) {
		t.Error("Secret condition should be satisfied when UseB=true")
	}
	if cond.Eval(map[string]any{"UseA": false, "UseB": false}) {
		t.Error("Secret condition should not be satisfied when both false")
	}
}

// Referenced-set tests

func TestReferenced_UnusedVarAbsent(t *testing.T) {
	// Unused is in project.yaml but never referenced in any template file.
	root := buildAnalysisTemplate(t,
		"Name: \"\"\nUnused: \"\"\n",
		map[string]string{"file.txt": "[[.Name]]"},
	)
	tmpl := analyzeTemplate(t, root)
	if tmpl.Referenced["Unused"] {
		t.Error("Unused should not be in Referenced")
	}
	if !tmpl.Referenced["Name"] {
		t.Error("Name should be in Referenced")
	}
}

func TestReferenced_ConditionalVarPresent(t *testing.T) {
	// DbName is only inside [[if .UseDB]] — still referenced.
	root := buildAnalysisTemplate(t,
		"UseDB: false\nDbName: \"\"\n",
		map[string]string{"file.txt": "[[if .UseDB]]DB=[[.DbName]][[end]]"},
	)
	tmpl := analyzeTemplate(t, root)
	if !tmpl.Referenced["DbName"] {
		t.Error("DbName should be in Referenced (conditional but still referenced)")
	}
}

func TestReferenced_ComputedOnlyVar(t *testing.T) {
	// Name is only referenced inside a computed expression, not in any template file.
	root := buildAnalysisTemplate(t,
		"Name: \"\"\ncomputed:\n  Upper: \"[[ toUpper .Name ]]\"\n",
		map[string]string{"file.txt": "[[.Upper]]"},
	)
	tmpl := analyzeTemplate(t, root)
	if !tmpl.Referenced["Name"] {
		t.Error("Name should be in Referenced (used in computed expression)")
	}
}
