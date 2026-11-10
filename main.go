package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	_ "modernc.org/sqlite"
)

type Tense struct {
	Name  string   `json:"name"`
	Forms []string `json:"forms"`
}

type Mood struct {
	Name   string  `json:"name"`
	Tenses []Tense `json:"tenses"`
}

type VerbEntry struct {
	Infinitive string
	Template   string
	Moods      []Mood
}

var elisionPrefixes = []string{
	"qu'elles'", "qu'ils'", "qu'elle'", "qu'il'", "qu'on'",
	"d'", "l'", "s'", "qu'", "j'", "n'", "m'", "t'",
}

func cleanInput(raw string) string {
	w := strings.TrimSpace(strings.ToLower(raw))
	w = strings.ReplaceAll(w, "’", "'")
	for _, p := range elisionPrefixes {
		if strings.HasPrefix(w, p) && len(w) > len(p) {
			w = w[len(p):]
			break
		}
	}
	return strings.Trim(w, ".,!?;:()-\"'")
}

func containsWordToken(line, target string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(line), "’", "'")
	tokens := strings.FieldsFunc(normalized, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\'' || r == '/' || r == '-'
	})
	for _, tok := range tokens {
		if tok == target {
			return true
		}
	}
	return false
}

func queryVerb(db *sql.DB, inf string) (*VerbEntry, error) {
	var tmpl, moodsJSON string
	err := db.QueryRow("SELECT template, moods_json FROM verbs WHERE infinitive = ?", inf).Scan(&tmpl, &moodsJSON)
	if err != nil {
		return nil, err
	}
	var moods []Mood
	if err := json.Unmarshal([]byte(moodsJSON), &moods); err != nil {
		return nil, err
	}
	return &VerbEntry{
		Infinitive: inf,
		Template:   tmpl,
		Moods:      moods,
	}, nil
}

func main() {
	if len(os.Args) < 2 {
		return
	}
	word := cleanInput(os.Args[1])
	if word == "" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}
	dbPath := filepath.Join(filepath.Dir(exePath), "verbs.db")

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return
	}
	defer db.Close()

	// Find all matching infinitives (exact infinitive match first, then inflected form matches)
	seen := make(map[string]bool)
	var infinitives []string

	var exactInf string
	if err := db.QueryRow("SELECT infinitive FROM verbs WHERE infinitive = ?", word).Scan(&exactInf); err == nil {
		seen[exactInf] = true
		infinitives = append(infinitives, exactInf)
	}

	rows, err := db.Query("SELECT infinitive FROM forms WHERE form = ? ORDER BY infinitive", word)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var inf string
			if err := rows.Scan(&inf); err == nil && !seen[inf] {
				seen[inf] = true
				infinitives = append(infinitives, inf)
			}
		}
	}

	if len(infinitives) == 0 {
		return
	}

	var entries []*VerbEntry
	for _, inf := range infinitives {
		if entry, err := queryVerb(db, inf); err == nil {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return
	}

	openByDefault := map[string]bool{
		"indicatif":    true,
		"subjonctif":   true,
		"conditionnel": true,
		"impératif":    true,
	}

	var sb strings.Builder
	sb.WriteString(`<style>
.gd-verbecc { font-family: -apple-system, BlinkMacSystemFont, sans-serif; font-size: 13px; line-height: 1.45; }
.gd-verbecc-verb-block { margin-bottom: 18px; }
.gd-verbecc-header { margin-bottom: 10px; padding-bottom: 6px; border-bottom: 1px solid rgba(128,128,128,0.3); }
.gd-verbecc-title { font-size: 16px; font-weight: 700; }
.gd-verbecc-meta { font-size: 12px; opacity: 0.7; margin-left: 8px; }
.gd-verbecc details { margin-bottom: 10px; border: 1px solid rgba(128,128,128,0.25); border-radius: 6px; padding: 6px 10px; }
.gd-verbecc summary { font-weight: 700; font-size: 14px; cursor: pointer; text-transform: capitalize; padding: 2px 0; }
.gd-verbecc-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 10px; margin-top: 8px; }
.gd-verbecc-tense { background: rgba(128,128,128,0.07); border-radius: 5px; padding: 8px 10px; }
.gd-verbecc-tense-title { font-weight: 600; font-size: 12.5px; text-transform: capitalize; margin-bottom: 5px; padding-bottom: 3px; border-bottom: 1px solid rgba(128,128,128,0.2); color: #3b82f6; }
.gd-verbecc-list { list-style: none; margin: 0; padding: 0; }
.gd-verbecc-list li { padding: 1px 0; }
.gd-verbecc-match { background: rgba(59,130,246,0.22); border-radius: 3px; padding: 1px 4px; font-weight: 600; }
</style>
<div class="gd-verbecc">`)

	for _, entry := range entries {
		escInf := html.EscapeString(entry.Infinitive)
		escTmpl := html.EscapeString(entry.Template)
		titleHTML := escInf
		if word != entry.Infinitive {
			titleHTML = fmt.Sprintf("%s &rarr; %s", html.EscapeString(word), escInf)
		}

		sb.WriteString(fmt.Sprintf(
			`<div class="gd-verbecc-verb-block"><div class="gd-verbecc-header"><span class="gd-verbecc-title">%s</span><span class="gd-verbecc-meta">modèle : %s</span></div>`,
			titleHTML, escTmpl,
		))

		for _, mood := range entry.Moods {
			openAttr := ""
			if openByDefault[strings.ToLower(mood.Name)] {
				openAttr = " open"
			}
			sb.WriteString(fmt.Sprintf(`<details%s><summary>%s</summary><div class="gd-verbecc-grid">`, openAttr, html.EscapeString(mood.Name)))

			for _, tense := range mood.Tenses {
				sb.WriteString(fmt.Sprintf(`<div class="gd-verbecc-tense"><div class="gd-verbecc-tense-title">%s</div><ul class="gd-verbecc-list">`, html.EscapeString(tense.Name)))
				for _, f := range tense.Forms {
					escForm := html.EscapeString(f)
					if word != entry.Infinitive && containsWordToken(f, word) {
						sb.WriteString(fmt.Sprintf(`<li class="gd-verbecc-match">%s</li>`, escForm))
					} else {
						sb.WriteString(fmt.Sprintf(`<li>%s</li>`, escForm))
					}
				}
				sb.WriteString(`</ul></div>`)
			}
			sb.WriteString(`</div></details>`)
		}
		sb.WriteString(`</div>`)
	}

	sb.WriteString(`</div>`)
	fmt.Println(sb.String())
}
