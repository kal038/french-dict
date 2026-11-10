#!/usr/bin/env python3
sqlite3 = __import__('sqlite3')
json = __import__('json')
logging = __import__('logging')
os = __import__('os')

logging.disable(logging.CRITICAL)
from verbecc import CompleteConjugator

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "verbs.db")

SKIP_TOKENS = {
    "je", "tu", "il", "elle", "on", "nous", "vous", "ils", "elles",
    "que", "ne", "me", "te", "se", "le", "la", "les", "en", "y"
}

PREFIXES = (
    "qu'elles'", "qu'ils'", "qu'elle'", "qu'il'", "qu'on'",
    "qu'", "j'", "s'", "m'", "t'", "n'", "l'", "d'"
)

def extract_verb_tokens(line: str, out_set: set):
    s = line.lower().replace("’", "'")
    for variant in s.split("/"):
        for raw_tok in variant.strip().split():
            tok = raw_tok
            changed = True
            while changed:
                changed = False
                for p in PREFIXES:
                    if tok.startswith(p) and len(tok) > len(p):
                        tok = tok[len(p):]
                        changed = True
                        break
            tok = tok.strip(".,!?;:()-\"'")
            if tok and tok not in SKIP_TOKENS:
                out_set.add(tok)

def get_all_infinitives(cg) -> list[str]:
    verbs = set()
    # verbecc 2.x stores loaded verbs inside internal grammar/lexicon structures
    # We can safely inspect cg or use its internal verb provider
    try:
        # Check standard attributes where verbecc indexes verbs
        sources = []
        if hasattr(cg, 'verbs'):
            sources.append(cg.verbs)
        if hasattr(cg, '_verbs'):
            sources.append(cg._verbs)
        if hasattr(cg, 'get_verbs'):
            sources.append(cg.get_verbs())
            
        for src in sources:
            if isinstance(src, (list, set)):
                for v in src:
                    if isinstance(v, str):
                        verbs.add(v)
                    elif hasattr(v, 'infinitive'):
                        verbs.add(str(v.infinitive))
            elif isinstance(src, dict):
                for k in src.keys():
                    verbs.add(str(k))
    except Exception:
        pass

    # Fallback / direct traversal if the attributes above missed anything
    if not verbs and hasattr(cg, 'grammar') and hasattr(cg.grammar, 'verbs'):
        for v in cg.grammar.verbs:
            if hasattr(v, 'infinitive'):
                verbs.add(str(v.infinitive))

    # If still empty, let's query a broad list of common French test prefixes or load from verbecc's dataset path
    if not verbs:
        import string
        for ch in string.ascii_lowercase + "àâéèêëîïôöùûüçœ":
            try:
                res = cg.get_verbs_that_start_with(ch)
                if res:
                    for v in res:
                        if isinstance(v, str):
                            verbs.add(v)
                        elif hasattr(v, 'infinitive'):
                            verbs.add(str(v.infinitive))
                        elif isinstance(v, dict) and 'infinitive' in v:
                            verbs.add(str(v['infinitive']))
            except Exception:
                pass

    return sorted(list(verbs))

def main():
    if os.path.exists(DB_PATH):
        os.remove(DB_PATH)

    conn = sqlite3.connect(DB_PATH)
    cur = conn.cursor()
    cur.execute("PRAGMA synchronous = OFF;")
    cur.execute("PRAGMA journal_mode = MEMORY;")

    cur.execute("""
        CREATE TABLE verbs (
            infinitive TEXT PRIMARY KEY,
            template TEXT,
            moods_json TEXT
        );
    """)
    cur.execute("""
        CREATE TABLE forms (
            form TEXT,
            infinitive TEXT,
            PRIMARY KEY (form, infinitive)
        );
    """)

    print("Loading verbecc French engine...")
    cg = CompleteConjugator("fr")
    
    # Direct extraction of all known French verbs from verbecc 2.x lexicon
    infinitives = []
    try:
        # In verbecc 2.x, cg.lexicon or cg.verbs contains the verb objects
        lexicon = getattr(cg, 'lexicon', None) or getattr(cg, 'verbs', None)
        if not lexicon and hasattr(cg, '_conjugator'):
            lexicon = getattr(cg._conjugator, 'lexicon', None)
            
        if lexicon:
            for item in lexicon:
                if isinstance(item, str):
                    infinitives.append(item)
                elif hasattr(item, 'infinitive'):
                    infinitives.append(str(item.infinitive))
                elif isinstance(item, dict) and 'infinitive' in item:
                    infinitives.append(str(item['infinitive']))
    except Exception:
        pass

    if not infinitives:
        infinitives = get_all_infinitives(cg)

    infinitives = sorted(list(set(infinitives)))
    print(f"Found {len(infinitives)} French verbs. Generating conjugations...")

    mood_order = ["indicatif", "subjonctif", "conditionnel", "impératif", "infinitif", "participe"]

    verb_rows = []
    form_rows = set()

    for idx, inf in enumerate(infinitives, 1):
        if idx % 500 == 0 or idx == len(infinitives):
            print(f"  Processed {idx}/{len(infinitives)} verbs...")
        try:
            res = cg.conjugate(inf)
            data = res.get_data() if hasattr(res, "get_data") else res
        except Exception:
            continue

        verb_info = data.get("verb", {})
        if verb_info.get("predicted", False):
            continue

        infinitive_str = str(verb_info.get("infinitive", inf)).strip().lower()
        template_str = str(verb_info.get("template", ""))
        moods = data.get("moods", {})
        if not moods:
            continue

        normalized_moods = {str(getattr(k, "value", k)): v for k, v in moods.items()}
        sorted_mood_names = [m for m in mood_order if m in normalized_moods] + [
            m for m in normalized_moods if m not in mood_order
        ]

        ordered_moods_payload = []
        verb_tokens = {infinitive_str}

        for m_name in sorted_mood_names:
            tenses = normalized_moods[m_name]
            if not isinstance(tenses, dict):
                continue
            tense_list = []
            for t_key, forms in tenses.items():
                t_name = str(getattr(t_key, "value", t_key))
                lines = []
                if isinstance(forms, list):
                    for item in forms:
                        if hasattr(item, "get_data"):
                            item = item.get_data()
                        if isinstance(item, dict):
                            c_list = item.get("c", [])
                            val = " / ".join(c_list) if c_list else ""
                        elif isinstance(item, (list, tuple)) and item:
                            val = str(item[0])
                        else:
                            val = str(item)
                        if val:
                            lines.append(val)
                            extract_verb_tokens(val, verb_tokens)
                elif isinstance(forms, str) and forms:
                    lines.append(forms)
                    extract_verb_tokens(forms, verb_tokens)
                tense_list.append({"name": t_name, "forms": lines})
            ordered_moods_payload.append({"name": m_name, "tenses": tense_list})

        verb_rows.append((
            infinitive_str,
            template_str,
            json.dumps(ordered_moods_payload, ensure_ascii=False, separators=(",", ":"))
        ))
        for tok in verb_tokens:
            form_rows.add((tok, infinitive_str))

    print(f"Inserting {len(verb_rows)} verbs and {len(form_rows)} inflected form mappings into SQLite...")
    cur.executemany("INSERT OR REPLACE INTO verbs VALUES (?, ?, ?);", verb_rows)
    cur.executemany("INSERT OR IGNORE INTO forms VALUES (?, ?);", list(form_rows))

    cur.execute("CREATE INDEX idx_forms_form ON forms(form);")
    conn.commit()
    cur.execute("VACUUM;")
    conn.close()
    size_mb = os.path.getsize(DB_PATH) / (1024 * 1024)
    print(f"Done! Created {DB_PATH} ({size_mb:.1f} MB)")

if __name__ == "__main__":
    main()