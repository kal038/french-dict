#!/usr/bin/env python3
import sys
import html
import logging

# Silence verbecc's default logger before importing
logging.disable(logging.CRITICAL)

from verbecc import CompleteConjugator
import simplemma

def resolve_verb(cg, word):
    """Try the raw word first; if not a known infinitive, lemmatize it."""
    candidates = [word]
    lemma = simplemma.lemmatize(word, lang='fr')
    if lemma and lemma.lower() != word:
        candidates.append(lemma.lower())

    for candidate in candidates:
        try:
            res = cg.conjugate(candidate)
            data = res.get_data()
            verb_info = data.get('verb', {})
            if not verb_info.get('predicted', False):
                return candidate, data
        except Exception:
            continue
    return None, None

def main():
    if len(sys.argv) < 2:
        sys.exit(0)

    word = sys.argv[1].strip().lower()
    # Strip leading elisions if copied from web text (e.g., "s'est", "qu'il", "d'aller")
    for prefix in ("d'", "d’", "l'", "l’", "s'", "s’", "qu'", "qu’", "j'", "j’", "n'", "n’", "m'", "m’", "t'", "t’"):
        if word.startswith(prefix) and len(word) > len(prefix):
            word = word[len(prefix):]
            break

    if not word:
        sys.exit(0)

    try:
        cg = CompleteConjugator('fr')
        resolved, data = resolve_verb(cg, word)
    except Exception:
        sys.exit(0)

    if not data:
        sys.exit(0)

    verb_info = data.get('verb', {})
    moods = data.get('moods', {})
    if not moods:
        sys.exit(0)

    infinitive = html.escape(str(verb_info.get('infinitive', resolved)))
    template = html.escape(str(verb_info.get('template', '')))

    title_html = (
        f'{html.escape(word)} &rarr; {infinitive}'
        if word != resolved else infinitive
    )

    out = [
        """<style>
        .gd-verbecc { font-family: -apple-system, BlinkMacSystemFont, sans-serif; font-size: 13px; line-height: 1.45; }
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
        .gd-verbecc-match { background: rgba(59,130,246,0.2); border-radius: 3px; padding: 1px 4px; font-weight: 600; }
        </style>""",
        '<div class="gd-verbecc">',
        f'<div class="gd-verbecc-header"><span class="gd-verbecc-title">{title_html}</span>'
        f'<span class="gd-verbecc-meta">modèle : {template}</span></div>'
    ]

    mood_order = ['indicatif', 'subjonctif', 'conditionnel', 'impératif', 'infinitif', 'participe']
    open_by_default = {'indicatif', 'subjonctif', 'conditionnel', 'impératif'}

    normalized_moods = {str(getattr(k, 'value', k)): v for k, v in moods.items()}
    sorted_moods = [m for m in mood_order if m in normalized_moods] + [
        m for m in normalized_moods if m not in mood_order
    ]

    for mood_name in sorted_moods:
        tenses = normalized_moods[mood_name]
        if not isinstance(tenses, dict):
            continue

        open_attr = ' open' if mood_name.lower() in open_by_default else ''
        out.append(f'<details{open_attr}><summary>{html.escape(mood_name)}</summary><div class="gd-verbecc-grid">')

        for tense_key, forms in tenses.items():
            tense_name = str(getattr(tense_key, 'value', tense_key))
            out.append(f'<div class="gd-verbecc-tense"><div class="gd-verbecc-tense-title">{html.escape(tense_name)}</div><ul class="gd-verbecc-list">')
            if isinstance(forms, list):
                for item in forms:
                    if hasattr(item, 'get_data'):
                        item = item.get_data()
                    if isinstance(item, dict):
                        c_list = item.get('c', [])
                        val = ' / '.join(c_list) if c_list else ''
                    elif isinstance(item, (list, tuple)) and item:
                        val = str(item[0])
                    else:
                        val = str(item)
                    if val:
                        escaped_val = html.escape(val)
                        # Highlight the exact conjugated form you searched inside the table!
                        if word != resolved and word in val.split():
                            out.append(f'<li class="gd-verbecc-match">{escaped_val}</li>')
                        else:
                            out.append(f'<li>{escaped_val}</li>')
            elif isinstance(forms, str):
                out.append(f'<li>{html.escape(forms)}</li>')
            out.append('</ul></div>')

        out.append('</div></details>')

    out.append('</div>')
    print('\n'.join(out))

if __name__ == '__main__':
    main()
