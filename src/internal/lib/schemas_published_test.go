package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/schemas"
)

// publishedSchemaBaseURL is the canonical, stable URL prefix under which the
// schemas are published. The /v1/ segment is a public contract: breaking
// changes must publish to a new /v2/ path. See schemas/README.md.
const publishedSchemaBaseURL = "https://raidcli.dev/schema/v1/"

// TestEmbeddedSchemas_haveCanonicalID asserts that every embedded schema has a
// $id under the published URL prefix. Catches accidental local-only $ids that
// would leave external validators unable to resolve cross-references.
func TestEmbeddedSchemas_haveCanonicalID(t *testing.T) {
	entries, err := schemas.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read embedded schemas: %v", err)
	}
	found := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".schema.json") {
			continue
		}
		found++
		data, err := schemas.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		id, _ := doc["$id"].(string)
		want := publishedSchemaBaseURL + name
		if id != want {
			t.Errorf("%s: $id = %q, want %q", name, id, want)
		}
	}
	if found == 0 {
		t.Fatal("no embedded schemas found")
	}
}

// TestPublishedSchemas_matchEmbedded asserts that if the docsite plugin has
// copied schemas into site/static/schema/v1/, those files byte-equal the
// embedded source. Catches drift between the canonical source and the
// published copy. Skipped when the static directory is absent (e.g. before a
// docs build has run locally).
func TestPublishedSchemas_matchEmbedded(t *testing.T) {
	root := repoRoot(t)
	publishedDir := filepath.Join(root, "site", "static", "schema", "v1")
	info, err := os.Stat(publishedDir)
	if err != nil || !info.IsDir() {
		t.Skipf("published schema dir not present (%s) — run docs build to generate", publishedDir)
	}

	entries, err := schemas.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read embedded schemas: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".schema.json") {
			continue
		}
		embedded, err := schemas.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		published, err := os.ReadFile(filepath.Join(publishedDir, name))
		if err != nil {
			t.Errorf("read published %s: %v", name, err)
			continue
		}
		if string(embedded) != string(published) {
			t.Errorf("%s: published copy differs from embedded source (rerun `npm run build` in site/)", name)
		}
	}
}

// TestEmbeddedTemplates_validateAgainstSchemas asserts that the profile and
// repo template files (rendered into a minimal valid form) validate against
// the embedded schemas. This guards against template/schema drift: if a
// future template adds a field that the schema doesn't permit, this test
// fails before users see it.
func TestEmbeddedTemplates_validateAgainstSchemas(t *testing.T) {
	root := repoRoot(t)

	tests := []struct {
		name        string
		template    string
		fill        string
		schemaID    string
		wantInvalid bool
	}{
		{
			name:     "profile-template",
			template: filepath.Join(root, "src/resources/profile-template"),
			// Templates ship with all data commented out. Append a minimal
			// schema-valid body so we exercise the rendered shape end-to-end.
			fill:     "\nname: tmpl-test\n",
			schemaID: "https://raidcli.dev/schema/v1/raid-profile.schema.json",
		},
		{
			name:     "repo-template",
			template: filepath.Join(root, "src/resources/repo-template"),
			fill:     "\nname: tmpl-test\nbranch: main\n",
			schemaID: "https://raidcli.dev/schema/v1/raid-repo.schema.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, err := os.ReadFile(tc.template)
			if err != nil {
				t.Fatalf("read template: %v", err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "rendered.yaml")
			if err := os.WriteFile(path, append(body, []byte(tc.fill)...), 0644); err != nil {
				t.Fatalf("write rendered: %v", err)
			}
			if err := validateWithEmbeddedSchema(path, tc.schemaID); err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
		})
	}
}

// TestPublishedExamples_validateAgainstSchemas asserts that every committed
// example under docs/examples/ validates against the appropriate schema.
// These are user-facing references — keeping them schema-valid catches PRs
// that break the example surface in lockstep with schema changes.
func TestPublishedExamples_validateAgainstSchemas(t *testing.T) {
	root := repoRoot(t)
	examplesDir := filepath.Join(root, "docs", "examples")

	cases := []struct {
		file     string
		schemaID string
	}{
		{"demo.raid.yaml", "https://raidcli.dev/schema/v1/raid-profile.schema.json"},
		{"env-demo.raid.yaml", "https://raidcli.dev/schema/v1/raid-profile.schema.json"},
		{"example.raid.yaml", "https://raidcli.dev/schema/v1/raid-profile.schema.json"},
		{"install-demo.raid.yaml", "https://raidcli.dev/schema/v1/raid-profile.schema.json"},
		{"multiple-profiles.yaml", "https://raidcli.dev/schema/v1/raid-profile.schema.json"},
		{"multiple-profiles.json", "https://raidcli.dev/schema/v1/raid-profile.schema.json"},
		{"raid.yaml", "https://raidcli.dev/schema/v1/raid-repo.schema.json"},
	}

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			path := filepath.Join(examplesDir, c.file)
			if _, err := os.Stat(path); err != nil {
				t.Skipf("example missing: %v", err)
			}
			if err := validateWithEmbeddedSchema(path, c.schemaID); err != nil {
				t.Errorf("%s: %v", c.file, err)
			}
		})
	}
}
