package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCleanInput verifies input trimming, elision removal, case normalization, and punctuation cleanup.
func TestCleanInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "standard infinitive", input: "manger", expected: "manger"},
		{name: "surrounding whitespace", input: "  aller  \n\t", expected: "aller"},
		{name: "uppercase normalization", input: "MANGER", expected: "manger"},
		{name: "mixed case with accents", input: "ÊtRe", expected: "être"},

		// French elision prefixes
		{name: "elision l'", input: "l'aimer", expected: "aimer"},
		{name: "elision d'", input: "d'avoir", expected: "avoir"},
		{name: "elision s'", input: "s'amuser", expected: "amuser"},
		{name: "elision c'", input: "c'est", expected: "est"},
		{name: "elision qu'", input: "qu'il", expected: "il"},
		{name: "elision j'", input: "j'aime", expected: "aime"},
		{name: "elision n'", input: "n'est", expected: "est"},
		{name: "elision m'", input: "m'appelle", expected: "appelle"},
		{name: "elision t'", input: "t'aime", expected: "aime"},
		{name: "longer elision qu'on'", input: "qu'on'sait", expected: "sait"},
		{name: "longer elision qu'elles'", input: "qu'elles'aiment", expected: "aiment"},

		// Curly apostrophes (common when pasting from web/mobile)
		{name: "curly apostrophe c’", input: "c’est", expected: "est"},
		{name: "curly apostrophe l’", input: "l’amour", expected: "amour"},
		{name: "curly apostrophe j’", input: "j’ai", expected: "ai"},
		{name: "curly apostrophe d’", input: "d’accord", expected: "accord"},
		{name: "curly apostrophe s’", input: "s’amuser", expected: "amuser"},

		// Punctuation stripping
		{name: "exclamation mark", input: "manger!", expected: "manger"},
		{name: "question mark", input: "aller?", expected: "aller"},
		{name: "parentheses", input: "(faire)", expected: "faire"},
		{name: "quotes", input: "\"partir\"", expected: "partir"},
		{name: "ellipsis and period", input: "...venir.", expected: "venir"},
		{name: "semicolon", input: "avoir;", expected: "avoir"},

		// Degenerate / boundary cases
		{name: "empty string", input: "", expected: ""},
		{name: "whitespace only", input: "   ", expected: ""},
		{name: "punctuation only", input: "...?!", expected: ""},
		{name: "elision only l'", input: "l'", expected: ""},
		{name: "elision only c'", input: "c'", expected: ""},
		{name: "curly elision only", input: "d’", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := cleanInput(tc.input)
			if actual != tc.expected {
				t.Errorf("cleanInput(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

// TestContainsWordToken verifies word boundary detection and token matching.
func TestContainsWordToken(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		target   string
		expected bool
	}{
		{name: "exact single word", line: "suis", target: "suis", expected: true},
		{name: "exact token in phrase", line: "je suis", target: "suis", expected: true},
		{name: "pronoun token", line: "je suis", target: "je", expected: true},
		{name: "substring should not match (prefix)", line: "je poursuis", target: "suis", expected: false},
		{name: "substring should not match (suffix)", line: "nous suivons", target: "suiv", expected: false},
		{name: "token with apostrophe", line: "j'ai", target: "ai", expected: true},
		{name: "token with curly apostrophe", line: "j’ai", target: "ai", expected: true},
		{name: "slash-separated variants", line: "allé / allée", target: "allée", expected: true},
		{name: "hyphenated token", line: "est-ce", target: "est", expected: true},
		{name: "hyphenated token right side", line: "est-ce", target: "ce", expected: true},
		{name: "case insensitive matching", line: "Je Suis", target: "suis", expected: true},
		{name: "target not present", line: "il chante", target: "mange", expected: false},
		{name: "empty line", line: "", target: "suis", expected: false},
		{name: "empty target", line: "je suis", target: "", expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := containsWordToken(tc.line, tc.target)
			if actual != tc.expected {
				t.Errorf("containsWordToken(%q, %q) = %v, want %v", tc.line, tc.target, actual, tc.expected)
			}
		})
	}
}

// TestFindDB verifies database resolution order and env overrides.
func TestFindDB(t *testing.T) {
	// 1. Explicit env override pointing to existing file
	tmpDir := t.TempDir()
	customDB := filepath.Join(tmpDir, "custom_verbs.db")
	if err := os.WriteFile(customDB, []byte("sqlite"), 0644); err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}

	origEnv := os.Getenv("CONJ_DB_PATH")
	defer os.Setenv("CONJ_DB_PATH", origEnv)

	os.Setenv("CONJ_DB_PATH", customDB)
	if got := findDB(); got != customDB {
		t.Errorf("findDB() with CONJ_DB_PATH = %q, want %q", got, customDB)
	}

	// 2. Explicit env override pointing to non-existent file should fall through
	os.Setenv("CONJ_DB_PATH", filepath.Join(tmpDir, "non_existent.db"))
	got := findDB()
	if got == filepath.Join(tmpDir, "non_existent.db") {
		t.Errorf("findDB() should ignore non-existent CONJ_DB_PATH")
	}

	// 3. Test fileExists helper
	if !fileExists(customDB) {
		t.Errorf("fileExists(%q) = false, want true", customDB)
	}
	if fileExists(filepath.Join(tmpDir, "does_not_exist")) {
		t.Errorf("fileExists for nonexistent file returned true")
	}
}

// setupMockDB creates an in-memory SQLite database populated with fixture data for testing.
func setupMockDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	schema := `
	CREATE TABLE verbs (
		infinitive TEXT PRIMARY KEY,
		template TEXT,
		moods_json TEXT
	);
	CREATE TABLE forms (
		form TEXT,
		infinitive TEXT,
		PRIMARY KEY (form, infinitive)
	);
	CREATE INDEX idx_forms_form ON forms(form);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	aimerMoods := `[
		{
			"name": "indicatif",
			"tenses": [
				{"name": "présent", "forms": ["j'aime", "tu aimes", "il aime", "nous aimons", "vous aimez", "ils aiment"]},
				{"name": "passé-composé", "forms": ["j'ai aimé", "tu as aimé"]}
			]
		},
		{
			"name": "infinitif",
			"tenses": [
				{"name": "infinitif-présent", "forms": ["aimer"]}
			]
		}
	]`

	etreMoods := `[
		{
			"name": "indicatif",
			"tenses": [
				{"name": "présent", "forms": ["je suis", "tu es", "il est", "nous sommes", "vous êtes", "ils sont"]}
			]
		}
	]`

	suivreMoods := `[
		{
			"name": "indicatif",
			"tenses": [
				{"name": "présent", "forms": ["je suis", "tu suis", "il suit", "nous suivons", "vous suivez", "ils suivent"]}
			]
		}
	]`

	insertVerb := "INSERT INTO verbs VALUES (?, ?, ?)"
	if _, err := db.Exec(insertVerb, "aimer", ":aimer", aimerMoods); err != nil {
		t.Fatalf("failed to insert aimer: %v", err)
	}
	if _, err := db.Exec(insertVerb, "être", ":être", etreMoods); err != nil {
		t.Fatalf("failed to insert être: %v", err)
	}
	if _, err := db.Exec(insertVerb, "suivre", ":suivre", suivreMoods); err != nil {
		t.Fatalf("failed to insert suivre: %v", err)
	}

	insertForm := "INSERT INTO forms VALUES (?, ?)"
	forms := [][2]string{
		{"aime", "aimer"},
		{"aimons", "aimer"},
		{"aimé", "aimer"},
		{"suis", "être"},
		{"suis", "suivre"},
		{"es", "être"},
		{"suit", "suivre"},
	}
	for _, f := range forms {
		if _, err := db.Exec(insertForm, f[0], f[1]); err != nil {
			t.Fatalf("failed to insert form %v: %v", f, err)
		}
	}

	return db
}

// TestQueryVerb tests querying and unmarshaling verb entries from SQLite.
func TestQueryVerb(t *testing.T) {
	db := setupMockDB(t)
	defer db.Close()

	// 1. Existing verb
	entry, err := queryVerb(db, "aimer")
	if err != nil {
		t.Fatalf("queryVerb('aimer') unexpected error: %v", err)
	}
	if entry.Infinitive != "aimer" {
		t.Errorf("entry.Infinitive = %q, want 'aimer'", entry.Infinitive)
	}
	if entry.Template != ":aimer" {
		t.Errorf("entry.Template = %q, want ':aimer'", entry.Template)
	}
	if len(entry.Moods) != 2 {
		t.Fatalf("len(entry.Moods) = %d, want 2", len(entry.Moods))
	}
	if entry.Moods[0].Name != "indicatif" {
		t.Errorf("first mood = %q, want 'indicatif'", entry.Moods[0].Name)
	}
	if len(entry.Moods[0].Tenses) != 2 {
		t.Fatalf("indicatif tenses = %d, want 2", len(entry.Moods[0].Tenses))
	}
	if entry.Moods[0].Tenses[0].Name != "présent" {
		t.Errorf("first tense = %q, want 'présent'", entry.Moods[0].Tenses[0].Name)
	}

	// 2. Non-existent verb
	_, err = queryVerb(db, "nonexistent")
	if err == nil {
		t.Errorf("queryVerb('nonexistent') expected error, got nil")
	}

	// 3. Corrupt JSON handling
	if _, err := db.Exec("INSERT INTO verbs VALUES (?, ?, ?)", "badjson", ":bad", "{invalid json"); err != nil {
		t.Fatalf("failed to insert badjson: %v", err)
	}
	_, err = queryVerb(db, "badjson")
	if err == nil {
		t.Errorf("queryVerb with invalid JSON expected unmarshal error, got nil")
	}
}

// TestFindMatchingInfinitives verifies exact match priority and reverse form lookup deduplication.
func TestFindMatchingInfinitives(t *testing.T) {
	db := setupMockDB(t)
	defer db.Close()

	// 1. Direct infinitive lookup
	infs, err := findMatchingInfinitives(db, "aimer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infs) != 1 || infs[0] != "aimer" {
		t.Errorf("findMatchingInfinitives('aimer') = %v, want ['aimer']", infs)
	}

	// 2. Form lookup that maps to multiple verbs
	infs, err = findMatchingInfinitives(db, "suis")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infs) != 2 {
		t.Fatalf("findMatchingInfinitives('suis') returned %d results, want 2", len(infs))
	}
	if (infs[0] != "être" && infs[0] != "suivre") || (infs[1] != "être" && infs[1] != "suivre") {
		t.Errorf("findMatchingInfinitives('suis') = %v, want ['être', 'suivre']", infs)
	}

	// 3. Unknown word
	infs, err = findMatchingInfinitives(db, "inconnu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infs) != 0 {
		t.Errorf("findMatchingInfinitives('inconnu') = %v, want empty", infs)
	}
}

// TestLookupVerbs tests full verb retrieval pipeline.
func TestLookupVerbs(t *testing.T) {
	db := setupMockDB(t)
	defer db.Close()

	entries, err := lookupVerbs(db, "suis")
	if err != nil {
		t.Fatalf("lookupVerbs('suis') error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("lookupVerbs('suis') returned %d entries, want 2", len(entries))
	}

	entries, err = lookupVerbs(db, "inconnu")
	if err != nil {
		t.Fatalf("lookupVerbs('inconnu') error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("lookupVerbs('inconnu') returned %d entries, want 0", len(entries))
	}
}

// TestRenderHTML verifies HTML structure, CSS, details tags, and match highlighting.
func TestRenderHTML(t *testing.T) {
	entries := []*VerbEntry{
		{
			Infinitive: "aimer",
			Template:   ":aimer",
			Moods: []Mood{
				{
					Name: "indicatif",
					Tenses: []Tense{
						{Name: "présent", Forms: []string{"j'aime", "tu aimes", "nous aimons"}},
					},
				},
				{
					Name: "infinitif",
					Tenses: []Tense{
						{Name: "infinitif-présent", Forms: []string{"aimer"}},
					},
				},
			},
		},
	}

	// 1. Empty entries
	if got := renderHTML("aimer", nil); got != "" {
		t.Errorf("renderHTML with nil entries = %q, want empty", got)
	}

	// 2. Exact infinitive lookup (word == infinitive)
	htmlInfinitive := renderHTML("aimer", entries)
	if !strings.Contains(htmlInfinitive, `<span class="gd-verbecc-title">aimer</span>`) {
		t.Errorf("infinitive header missing simple title: %s", htmlInfinitive)
	}
	if strings.Contains(htmlInfinitive, "&rarr;") {
		t.Errorf("infinitive search should not contain arrow &rarr;")
	}
	if strings.Contains(htmlInfinitive, `<li class="gd-verbecc-match">`) {
		t.Errorf("infinitive search should not highlight forms")
	}
	if !strings.Contains(htmlInfinitive, `<details open><summary>indicatif</summary>`) {
		t.Errorf("indicatif mood should have 'open' attribute")
	}
	if !strings.Contains(htmlInfinitive, `<details><summary>infinitif</summary>`) {
		t.Errorf("infinitif mood should NOT have 'open' attribute")
	}

	// 3. Conjugated form lookup (word != infinitive) -> should show arrow and highlight matching form
	htmlForm := renderHTML("aimons", entries)
	if !strings.Contains(htmlForm, `<span class="gd-verbecc-title">aimons &rarr; aimer</span>`) {
		t.Errorf("form search missing arrow header: %s", htmlForm)
	}
	if !strings.Contains(htmlForm, `<li class="gd-verbecc-match">nous aimons</li>`) {
		t.Errorf("matching form 'nous aimons' was not highlighted with gd-verbecc-match: %s", htmlForm)
	}
	if !strings.Contains(htmlForm, `<li>j&#39;aime</li>`) {
		t.Errorf("non-matching form 'j&#39;aime' should be plain <li>: %s", htmlForm)
	}

	// 4. HTML escaping safety
	unsafeEntry := []*VerbEntry{
		{
			Infinitive: "<script>alert('xss')</script>",
			Template:   "<b>model</b>",
			Moods: []Mood{
				{
					Name: "mood&safe",
					Tenses: []Tense{
						{Name: "tense<1>", Forms: []string{"form & value"}},
					},
				},
			},
		},
	}
	unsafeHTML := renderHTML("<script>", unsafeEntry)
	if strings.Contains(unsafeHTML, "<script>alert") {
		t.Errorf("unescaped script tag in HTML output: %s", unsafeHTML)
	}
	if !strings.Contains(unsafeHTML, "&lt;script&gt;alert") {
		t.Errorf("script tag was not properly escaped: %s", unsafeHTML)
	}
	if !strings.Contains(unsafeHTML, "mood&amp;safe") {
		t.Errorf("ampersand in mood was not escaped: %s", unsafeHTML)
	}
}

// TestRun verifies the full execution pipeline with various CLI argument inputs.
func TestRun(t *testing.T) {
	// 1. Missing arguments -> no output, no error
	var buf bytes.Buffer
	if err := run([]string{"conj"}, &buf); err != nil {
		t.Errorf("run with no args returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("run with no args produced output: %q", buf.String())
	}

	// 2. Blank or punctuation argument -> no output, no error
	buf.Reset()
	if err := run([]string{"conj", "   "}, &buf); err != nil {
		t.Errorf("run with whitespace returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("run with whitespace produced output: %q", buf.String())
	}

	buf.Reset()
	if err := run([]string{"conj", "..."}, &buf); err != nil {
		t.Errorf("run with punctuation returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("run with punctuation produced output: %q", buf.String())
	}

	// 3. Missing database path -> no output, no error
	tmpDir := t.TempDir()
	origEnv := os.Getenv("CONJ_DB_PATH")
	defer os.Setenv("CONJ_DB_PATH", origEnv)

	os.Setenv("CONJ_DB_PATH", filepath.Join(tmpDir, "missing.db"))
	buf.Reset()
	if err := run([]string{"conj", "aimer"}, &buf); err != nil {
		t.Errorf("run with missing db returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("run with missing db produced output: %q", buf.String())
	}

	// 4. Valid database via CONJ_DB_PATH
	testDBPath := filepath.Join(tmpDir, "test_verbs.db")
	db, err := sql.Open("sqlite", testDBPath)
	if err != nil {
		t.Fatalf("failed to open test db file: %v", err)
	}
	schema := `
	CREATE TABLE verbs (infinitive TEXT PRIMARY KEY, template TEXT, moods_json TEXT);
	CREATE TABLE forms (form TEXT, infinitive TEXT, PRIMARY KEY (form, infinitive));
	INSERT INTO verbs VALUES ('parler', ':parler', '[{"name":"indicatif","tenses":[{"name":"présent","forms":["je parle","nous parlons"]}]}]');
	INSERT INTO forms VALUES ('parle', 'parler');
	INSERT INTO forms VALUES ('parlons', 'parler');
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("failed to create test db schema: %v", err)
	}
	db.Close()

	os.Setenv("CONJ_DB_PATH", testDBPath)

	// Lookup infinitive
	buf.Reset()
	if err := run([]string{"conj", "parler"}, &buf); err != nil {
		t.Fatalf("run('parler') returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "parler") || !strings.Contains(buf.String(), "je parle") {
		t.Errorf("run('parler') output missing expected content: %s", buf.String())
	}

	// Lookup reverse form with elision: "nous parlons" -> "parlons"
	buf.Reset()
	if err := run([]string{"conj", "parlons"}, &buf); err != nil {
		t.Fatalf("run('parlons') returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "parlons &rarr; parler") {
		t.Errorf("run('parlons') missing arrow header: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "gd-verbecc-match") {
		t.Errorf("run('parlons') missing match highlight: %s", buf.String())
	}

	// Unknown word -> no output
	buf.Reset()
	if err := run([]string{"conj", "inconnu"}, &buf); err != nil {
		t.Fatalf("run('inconnu') returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("run('inconnu') expected empty output, got: %s", buf.String())
	}
}

// TestIntegrationWithRealDB tests against the local verbs.db if present.
func TestIntegrationWithRealDB(t *testing.T) {
	// Look for verbs.db in workspace root first, or via findDB()
	absDB, err := filepath.Abs("verbs.db")
	if err == nil && fileExists(absDB) {
		origEnv := os.Getenv("CONJ_DB_PATH")
		defer os.Setenv("CONJ_DB_PATH", origEnv)
		os.Setenv("CONJ_DB_PATH", absDB)
	}

	dbPath := findDB()
	if dbPath == "" || !fileExists(dbPath) {
		t.Skip("verbs.db not found, skipping real DB integration tests")
	}

	tests := []struct {
		name         string
		arg          string
		mustContain  []string
		mustNotMatch []string
	}{
		{
			name:        "infinitive avoir",
			arg:         "avoir",
			mustContain: []string{"<span class=\"gd-verbecc-title\">avoir</span>", "j&#39;ai", "nous avons", "indicatif", "subjonctif"},
		},
		{
			name:        "infinitive être",
			arg:         "être",
			mustContain: []string{"<span class=\"gd-verbecc-title\">être</span>", "je suis", "nous sommes"},
		},
		{
			name:        "reverse lookup suis (maps to être and suivre)",
			arg:         "suis",
			mustContain: []string{"suis &rarr; être", "suis &rarr; suivre", "gd-verbecc-match"},
		},
		{
			name:        "elision c'est",
			arg:         "c'est",
			mustContain: []string{"est &rarr; être", "gd-verbecc-match"},
		},
		{
			name:        "curly elision c’est",
			arg:         "c’est",
			mustContain: []string{"est &rarr; être", "gd-verbecc-match"},
		},
		{
			name:        "elision j'aime",
			arg:         "j'aime",
			mustContain: []string{"aime &rarr; aimer", "gd-verbecc-match"},
		},
		{
			name:        "irregular past tense eut",
			arg:         "eut",
			mustContain: []string{"eut &rarr; avoir", "gd-verbecc-match"},
		},
		{
			name:        "irregular subjunctive fasse",
			arg:         "fasse",
			mustContain: []string{"fasse &rarr; faire", "gd-verbecc-match"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := run([]string{"conj", tc.arg}, &buf); err != nil {
				t.Fatalf("run(%q) error: %v", tc.arg, err)
			}
			out := buf.String()
			for _, exp := range tc.mustContain {
				if !strings.Contains(out, exp) {
					t.Errorf("run(%q) missing expected substring: %q", tc.arg, exp)
				}
			}
		})
	}

	// Verify unknown word produces no output
	t.Run("non-existent word", func(t *testing.T) {
		var buf bytes.Buffer
		if err := run([]string{"conj", "mottotalementinconnu12345"}, &buf); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output for unknown word, got: %s", buf.String())
		}
	})
}
