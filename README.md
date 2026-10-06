# mentor-client 💻🎒

> **The official lightweight developer CLI for the Mentor Platform.**  
> Code natively on your machine, inspect architectural briefs in your terminal, and verify your implementation against the server's authoritative Dynamic Dual Oracle with zero client-side leaks.

---

## 🌟 Overview & Key Principles

`mentor-client` gives learners an authentic, real-world development experience:

1. **Native Local Development:** No toy in-browser web editors. Write real code in real project folders on your own workstation using your preferred editor (VS Code, GoLand, Neovim, Zed).
2. **Pure Standard Library & Portability:** Written in pure Go with zero external dependencies (no Git, no CGO, no heavy frameworks). A single, lightweight, highly portable binary.
3. **Zero-Leak Anti-Cheat Invariant:** The client workspace contains **zero test suites, zero hidden edge cases, and zero answer keys**. Authoritative verification occurs against the server (or local authoritative spec) which returns only clean, projected, spoiler-free diagnostics.
4. **Dynamic Dual Oracle Anti-Cheat:** Submissions execute through a two-phase evaluation pipeline:
   - **Gate 1 (60%):** Predefined behavioral contract tests verifying core requirements.
   - **Gate 2 (40%):** Dynamic Differential Oracle fuzzing against the golden reference solution, eliminating hardcoded cheats and edge-case guessing.
   - **Demo Cases:** Canonical input/output smoke test pairs verifying input handling.
5. **Flexible Output Matchers:** Supports `exact`, `contains`, `trim`, `prefix`, `suffix`, and `unordered_lines` (set-based multi-line comparison for concurrent or unordered program outputs).
6. **Polyglot Execution:** Native runner support for Go (`go run .`), Python (`python3 main.py`), and Rust (`cargo run --quiet`).
7. **Dual-Mode Operation:** Works seamlessly in both **Local Offline Mode** (evaluating against local curriculum directories via `--course` / `--local`) and **Server Mode** (submitting to remote `mentor-server` instances).

---

## 🛠️ Installation & Build

Build directly from source using Go (1.21+):

```bash
git clone https://github.com/upiottxat/mentor-client.git
cd mentor-client
go build -o mentor-client ./cmd/mentor-client
```

Optionally move the binary to your system `$PATH`:

```bash
sudo mv mentor-client /usr/local/bin/
```

Verify installation:

```bash
mentor-client help
```

---

## 🚀 CLI Commands & Workflow

```
mentor-client <command> [flags]

COMMANDS:
  init            Initialize an isolated course workspace with verified starter code
  status          Inspect active level, progression metrics, and milestone goals
  brief           Render architectural brief, public constraints, and verification contract
  hint            Request escalating progressive hint tier without solution leakage
  submit          Submit code to authoritative runner (triggers Dynamic Dual Oracle)
  help            Display client commands and configuration options
```

---

### 1. `init` — Initialize Course Workspace

Scaffolds a new local development directory with verified starter code and workspace tracking metadata (`.mentor/config.json`).

#### Local Mode (Directly from a generated curriculum folder):
```bash
mentor-client init --course /path/to/mini-project-course [--dir ./my-shell]
# Or using the shorthand alias:
mentor-client init --local /path/to/mini-project-course
```

#### Server Mode (Connected to a remote grading server):
```bash
mentor-client init simple-shell --server http://localhost:8080 [--dir ./my-shell]
```

**What happens during `init`:**
- Automatically detects the course slug, target language, and starter code templates.
- Extracts starter files (`main.go`, `go.mod`, etc.) into your destination directory.
- Initializes `.mentor/config.json` tracking your active level and course configuration.
- Prints onboarding kickstart guidance for Level 1.

---

### 2. `brief` / `doc` — Inspect Level Requirements

Displays the architectural goals and constraints of the active level directly in formatted terminal output:

```bash
cd my-shell
mentor-client brief
```

Or target a specific level or course path explicitly:
```bash
mentor-client brief --course /path/to/mini-project-course --level 1
```

**Displayed Sections:**
- **Level Header:** Title, level number, and core pedagogical objective.
- **Pedagogical Motivation & Hook:** Why this stage exists and what breaks downstream if omitted.
- **Concrete Demonstration:** Illustrative user/operator interaction scenario.
- **Required Behaviors:** Explicit behavioral requirements (`B1`, `B2`, etc.).
- **Public Constraints:** Language constraints, disallowed packages, or memory/CPU limits.

---

### 3. `status` — View Progress & Workspace Details

Displays course information, active level, target language, and server connection status:

```bash
mentor-client status
```

---

### 4. `hint` — Request Progressive Hints

If you are stuck and need a nudge without spoiling the solution, request tiered hints:

```bash
mentor-client hint [tier]
```

- **Tier 1 (Conceptual):** Clarifies the underlying mental model and domain concepts.
- **Tier 2 (Directional):** Points toward relevant standard library functions or patterns.
- **Tier 3 (Structural):** Outlines algorithmic structure and control flow without revealing the implementation.

---

### 5. `submit` — Test & Advance Level

Packages your local source files and runs verification against the **Dynamic Dual Oracle**:

```bash
mentor-client submit
```

Or target specific levels or directories:
```bash
mentor-client submit --course /path/to/mini-project-course --level 1 --dir ./my-shell
```

**The Verification Pipeline:**
1. Packages all non-ignored source files in the workspace.
2. In **Local Mode**:
   - Compiles and runs your code in your workspace directory.
   - Evaluates Predefined Tests (60%) and Dynamic Oracle / Demo Cases (40%).
   - Applies strict output matchers (`exact`, `trim`, `contains`, `prefix`, `suffix`, `unordered_lines`).
3. In **Server Mode**:
   - Dispatches code to the authoritative `mentor-server` sandbox runner.
   - Evaluates differentially against the server's golden reference solution.
4. **On Pass:** Automatically increments your active level (`CurrentLevel++`) in `.mentor/config.json` and unlocks the next level's brief!

---

## 🗂️ Workspace Structure

A `mentor-client` workspace has a clean, standard layout:

```
my-shell/
├── .mentor/
│   └── config.json       # Workspace tracking: course slug, active level, language, server URL
├── main.go               # Your source code
├── go.mod                # Module definitions
└── ...                   # Additional source files as your architecture grows
```

`.mentor/config.json` format:
```json
{
  "local_path": "/path/to/mini-project-course",
  "server_url": "http://localhost:8080",
  "course_slug": "mini-project-course",
  "current_level": 1,
  "language": "go",
  "student_token": ""
}
```

---

## 🧪 Testing

Run the test suite across the client repository:

```bash
go test -v ./...
```

---

## 🛡️ License

This project is licensed under the MIT License.
