# mentor-client 💻🎒

The official lightweight student CLI for the **Mentor Platform**.

`mentor-client` provides learners with an authentic local development experience. You code directly on your local machine using your favorite text editor (VS Code, GoLand, Neovim), inspect level goals and constraints directly in your terminal, and submit code to the authoritative grading server with zero leaks and real-time feedback.

---

## 🌟 Key Principles

1. **Native Local Development:** No toy web editors. Write real code in real project folders on your own machine.
2. **Pure Go Standard Library:** Zero external dependencies (no Git, no CGO, no heavy frameworks). Single, self-contained portable binary.
3. **"Correctness-Goonga" Zero-Leak Invariant:** The client contains zero test suites, zero hidden edge cases, and zero answer keys. The grading server executes authoritative verification and returns only clean, projected, spoiler-free diagnostics.
4. **Dynamic Dual Oracle Anti-Cheat:** Submissions trigger a two-phase verification pipeline:
   - **Gate 1 (60%):** Predefined behavioral contract tests.
   - **Gate 2 (40%):** Dynamic Differential Oracle fuzzing against the hidden reference solution to eliminate hardcoded cheats.

---

## 🛠️ Installation & Build

Build directly using Go (1.21+):

```bash
git clone https://github.com/upiottxat/mentor-client.git
cd mentor-client
go build -o mentor-client ./cmd/mentor-client
```

Optionally move to your `$PATH`:
```bash
mv mentor-client /usr/local/bin/
```

---

## 🚀 Quickstart & Workflow

### 1. Initialize a Course Workspace
```bash
# Connect to a live mentor-server:
mentor-client init simple-shell --server http://localhost:8080

# Or initialize offline from a local curriculum directory:
mentor-client init simple-shell --local /path/to/curriculum-output
```

This creates a local workspace folder with:
- Initial starter code templates (`main.go`, `go.mod`, etc.)
- `.mentor/config.json` tracking your active progress.

### 2. Inspect the Active Level Brief
```bash
cd simple-shell
mentor-client brief
```
Displays the level title, goal, pedagogical motivation, expected demonstration, public constraints, and required behaviors in formatted terminal output.

### 3. Check Workspace Status
```bash
mentor-client status
```
Shows active course, current level, and server connection details.

### 4. Request Progressive Hints
If you are stuck and need a gentle push:
```bash
mentor-client hint
```
Provides progressive hint tiers (Conceptual $\to$ Directional $\to$ Structural) without spoiling the solution.

### 5. Submit Your Solution
```bash
mentor-client submit
```
Packages your source files and triggers the **Dynamic Dual Oracle** on the server. On pass, advances your workspace to the next level automatically!

---

## 🧪 Testing

Run all unit tests:
```bash
go test -v ./...
```
