# French Verb Conjugation Dictionary for GoldenDict-ng

A fast, offline French verb conjugator that integrates with [GoldenDict-ng](https://github.com/xiaoyifang/goldendict-ng) as an external program dictionary. Look up any French verb form — conjugated or infinitive — and instantly see full conjugation tables with all moods and tenses, beautifully rendered in HTML.

**Features:**

- **Reverse lookup** — type any conjugated form (e.g. _suis_, _allées_, _eût_) and find which verb(s) it belongs to, with the matching form highlighted
- **Complete conjugation tables** — indicatif, subjonctif, conditionnel, impératif, infinitif, participe
- **Instant results** — pre-built SQLite database + compiled Go binary = sub-millisecond lookups
- **Clean HTML output** — responsive grid layout with collapsible mood sections, styled for both light and dark themes
- **Smart input cleaning** — automatically strips French elisions (`l'`, `s'`, `j'`, `qu'`, etc.)
- **Cross-platform** — works on macOS (M chips/Intel)/Windows/Linux (ARM/x86)

## How It Works

```
┌──────────────┐     ┌────────────────┐     ┌──────────────┐     ┌─────────────────┐
│ GoldenDict-ng│────▶│  conj binary   │────▶│   verbs.db   │────▶│  HTML output     │
│  (looks up   │     │  (Go, reads    │     │  (SQLite,    │     │  (conjugation    │
│   a word)    │     │   CLI arg)     │     │   ~65 MB)    │     │   tables)        │
└──────────────┘     └────────────────┘     └──────────────┘     └─────────────────┘
```

1. **`build_db.py`** uses the [verbecc](https://github.com/bretttolbert/verbecc) library to conjugate every known French verb and stores all conjugations + a reverse-lookup index of inflected forms into `verbs.db` (SQLite).
2. **`conj`** (compiled from `main.go`) is a standalone binary that takes a word as a CLI argument, queries `verbs.db`, and prints styled HTML to stdout.
3. **GoldenDict-ng** calls `conj` as an external program and renders the HTML output inline alongside your other dictionaries.

## Prerequisites

| Tool                             | macOS                               | Windows                                                                                  |
| -------------------------------- | ----------------------------------- | ---------------------------------------------------------------------------------------- |
| **Go** ≥ 1.21                    | `brew install go`                   | [Download installer](https://go.dev/dl/)                                                 |
| **Python** ≥ 3.10                | `brew install python`               | [Download installer](https://www.python.org/downloads/)                                  |
| **uv** (Python package manager)  | `brew install uv`                   | `pip install uv` or [installer](https://docs.astral.sh/uv/getting-started/installation/) |
| **just** (optional, task runner) | `brew install just`                 | `cargo install just` or [prebuilt binaries](https://github.com/casey/just/releases)      |
| **GoldenDict-ng**                | `brew install --cask goldendict-ng` | [Download installer](https://github.com/xiaoyifang/goldendict-ng/releases)               |

## Setup

### Step 1: Clone the Repository

```bash
git clone <your-repo-url> Dictionaries
cd Dictionaries
```

### Step 2: Build the Verb Database

This generates `verbs.db` (~65 MB) containing all French verb conjugations and a reverse-lookup index.

**Using `just` (recommended):**

```bash
just regen-db
```

**Or manually:**

```bash
# Create a virtual environment and install verbecc
uv venv .venv

# macOS / Linux
uv pip install --python .venv/bin/python verbecc
./.venv/bin/python build_db.py

# Windows (PowerShell)
uv pip install --python .venv\Scripts\python.exe verbecc
.\.venv\Scripts\python.exe build_db.py
```

This takes a few minutes. You'll see progress output like:

```
Loading verbecc French engine...
Found 7745 French verbs. Generating conjugations...
  Processed 500/7745 verbs...
  ...
Done! Created verbs.db (65.1 MB)
```

### Step 3: Build the Go Binary

**Using `just`:**

```bash
just build
```

**Or manually:**

```bash
# macOS / Linux
go build -ldflags="-s -w" -o conj main.go

# Windows (PowerShell)
go build -ldflags="-s -w" -o conj.exe main.go
```

> [!IMPORTANT]
> The `conj` binary and `verbs.db` must be in the **same directory**. The binary locates the database relative to its own path at runtime.

### Step 4: Verify It Works

```bash
# macOS / Linux
./conj manger

# Windows
.\conj.exe manger
```

You should see HTML output with the full conjugation table. Try a conjugated form too:

```bash
./conj mangeons   # → finds "manger" and highlights "mangeons"
./conj suis       # → finds "être" and "suivre"
```

## Integrating with GoldenDict-ng

### macOS

1. Open **GoldenDict-ng** → **Preferences** (⌘ + ,) → **Sources** → **Programs** tab.
2. Click **Add** to create a new entry.
3. Configure the fields:

   | Field       | Value                                       |
   | ----------- | ------------------------------------------- |
   | **Enabled** | ✅                                          |
   | **Type**    | `Html`                                      |
   | **Name**    | `French Conjugation` (or any name you like) |
   | **Command** | `/full/path/to/Dictionaries/conj %GDWORD%`  |
   | **Icon**    | _(optional)_                                |

4. Click **OK** to save.

> [!TIP]
> To find the full path, run `pwd` in the Dictionaries directory, then append `/conj`. For example:
>
> ```
> /Users/yourname/Dictionaries/conj %GDWORD%
> ```

### Windows

1. Open **GoldenDict-ng** → **Edit** → **Preferences** → **Sources** → **Programs** tab.
2. Click **Add** to create a new entry.
3. Configure the fields:

   | Field       | Value                                            |
   | ----------- | ------------------------------------------------ |
   | **Enabled** | ✅                                               |
   | **Type**    | `Html`                                           |
   | **Name**    | `French Conjugation`                             |
   | **Command** | `C:\full\path\to\Dictionaries\conj.exe %GDWORD%` |

4. Click **OK** to save.

> [!NOTE]
> Use the full absolute path to `conj.exe`. Relative paths may not resolve correctly in GoldenDict-ng on Windows.

### Verify the Integration

1. In GoldenDict-ng, type any French verb or conjugated form in the search bar (e.g., `avoir`, `suis`, `mangeait`).
2. You should see a styled conjugation table appear in the results, alongside entries from your other dictionaries.
3. If the looked-up word is a conjugated form (not the infinitive), the matching form will be highlighted in blue in the table.

## Cross-Compiling for Other Platforms

The Go binary can be cross-compiled for any platform without needing a C compiler, since it uses a pure-Go SQLite driver (`modernc.org/sqlite`).

```bash
# Windows (from macOS/Linux)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o conj.exe main.go

# Linux ARM64 (e.g., Raspberry Pi, cloud VMs)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o conj-linux-arm64 main.go

# Linux x86_64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o conj-linux-amd64 main.go

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o conj-darwin-amd64 main.go
```

Copy both the compiled binary **and** `verbs.db` to the target machine.

## Alternative: Python-Only Mode (`conj.py`)

If you don't want to pre-build the database, `conj.py` is a standalone Python script that conjugates verbs on-the-fly using `verbecc` and `simplemma`. It's slower (~2-3 seconds per lookup vs. <10ms for the Go binary) but requires no build step.

### Setup

```bash
uv venv .venv

# macOS / Linux
uv pip install --python .venv/bin/python verbecc simplemma

# Windows
uv pip install --python .venv\Scripts\python.exe verbecc simplemma
```

### GoldenDict-ng Configuration

| Field                 | Value                                                                                                 |
| --------------------- | ----------------------------------------------------------------------------------------------------- |
| **Type**              | `Html`                                                                                                |
| **Name**              | `French Conjugation (Python)`                                                                         |
| **Command (macOS)**   | `/full/path/to/Dictionaries/.venv/bin/python /full/path/to/Dictionaries/conj.py %GDWORD%`             |
| **Command (Windows)** | `C:\full\path\to\Dictionaries\.venv\Scripts\python.exe C:\full\path\to\Dictionaries\conj.py %GDWORD%` |

> [!WARNING]
> The Python version loads the verbecc engine on every invocation, so there is a noticeable delay (~2-3s) per lookup. The Go binary + SQLite approach is **strongly recommended** for daily use.

## Morphology Support

The `Morphology/` directory contains placeholder files for French Hunspell morphology (`fr_FR.aff`, `fr_FR.dic`). GoldenDict-ng can use Hunspell morphology files to expand lookups to related word forms.

To enable this in GoldenDict-ng:

1. Download French Hunspell dictionaries (e.g., from [LibreOffice dictionaries](https://github.com/LibreOffice/dictionaries/tree/master/fr_FR)).
2. Place `fr_FR.aff` and `fr_FR.dic` in the `Morphology/` directory.
3. In GoldenDict-ng → **Preferences** → **Sources** → **Morphology** tab, add the `Morphology/` directory.

## Project Structure

```
Dictionaries/
├── main.go          # Go binary source — CLI lookup against verbs.db
├── go.mod / go.sum  # Go module dependencies
├── build_db.py      # Python script — generates verbs.db from verbecc
├── conj.py          # Python script — standalone on-the-fly conjugator
├── conj             # Compiled Go binary (macOS ARM64, not committed)
├── verbs.db         # SQLite database with all conjugations (generated)
├── Justfile         # Task runner recipes (build, regen-db)
├── Morphology/      # Hunspell morphology files for GoldenDict-ng
│   ├── fr_FR.aff
│   └── fr_FR.dic
└── verbecc.log      # Build log from verbecc engine initialization
```

## Troubleshooting

| Problem                            | Solution                                                                                                                                |
| ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `conj` outputs nothing             | Ensure `verbs.db` is in the same directory as the `conj` binary. Run `./conj avoir` directly to test.                                   |
| No results in GoldenDict-ng        | Check that the **Type** is set to `Html` (not `Plain Text`). Verify the full path in the command is correct.                            |
| `verbs.db` not found               | Run `just regen-db` or follow Step 2 to rebuild the database.                                                                           |
| Permission denied (macOS)          | Run `chmod +x conj` to make the binary executable. On macOS, you may also need to allow it in **System Settings → Privacy & Security**. |
| Windows Defender blocks `conj.exe` | Add an exception for the binary, or run `go build` locally on the Windows machine so it isn't flagged as an unknown download.           |
| Python `conj.py` is slow           | This is expected (~2-3s). Use the Go binary + `verbs.db` for fast lookups.                                                              |
| `uv` not found                     | Install with `brew install uv` (macOS) or `pip install uv` (Windows).                                                                   |

## License

The verb conjugation data is generated by [verbecc](https://github.com/bretttolbert/verbecc) (GPL-3.0).
