package tmpl

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// importSpec matches the specifier of a static `import ... from '...'`, a bare
// `import '...'` and a dynamic `import('...')`, plus app.js's component table
// entries (['./components/x.js', 'initX']), which are fed to import().
var importSpec = regexp.MustCompile(`(?:from\s+|import\s*\(?\s*|\[\s*)'(\.{1,2}/[^']+\.js)'`)

// TestImportMapCoversEveryModule guards the cache-busting import map: every
// module specifier used anywhere in public/ JS must resolve to a map key, or
// that module would load from its plain (unversioned) URL and could be served
// stale. Also checks each entry is content-hashed.
func TestImportMapCoversEveryModule(t *testing.T) {
	// Renderer paths are relative to the project root.
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	r := &Renderer{}
	tag := string(r.importMap())
	const open, close = `<script type="importmap">`, `</script>`
	if !strings.HasPrefix(tag, open) || !strings.HasSuffix(tag, close) {
		t.Fatalf("importMap() = %q, want a <script type=importmap> tag", tag)
	}
	var m struct{ Imports map[string]string }
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(tag, open), close)), &m); err != nil {
		t.Fatalf("import map is not valid JSON: %v", err)
	}
	for k, v := range m.Imports {
		if !regexp.MustCompile(`^` + regexp.QuoteMeta(k) + `\?v=[0-9a-f]{8}$`).MatchString(v) {
			t.Errorf("map entry %q -> %q is not content-hashed", k, v)
		}
	}

	var files []string
	for _, g := range jsModuleGlobs {
		fs, _ := filepath.Glob(g)
		files = append(files, fs...)
	}
	checked := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		dir := "/" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(f, "public/")))
		for _, sm := range importSpec.FindAllStringSubmatch(string(src), -1) {
			resolved := path.Clean(path.Join(dir, sm[1]))
			checked++
			if _, ok := m.Imports[resolved]; !ok {
				t.Errorf("%s imports %q (→ %s), which is not in the import map", f, sm[1], resolved)
			}
		}
	}
	// Sanity: the regex must actually be finding the imports (app.js alone has 14).
	if checked < 15 {
		t.Errorf("only %d import specifiers found — importSpec regex is probably broken", checked)
	}
}
