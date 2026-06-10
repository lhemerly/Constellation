## 2026-06-10 - Slice Allocation in Loop Optimizations
**Learning:** In Go, continuously appending to a dynamically growing slice (like flattening a list of byte slices in `MapReduceNode`) causes repeated memory allocations. Pre-calculating the total required length and initializing the slice with `make([]byte, 0, totalLen)` is a measurable and effective optimization.
**Action:** When implementing or modifying nodes that aggregate data (e.g., `ScatterGatherNode`, `MapReduceNode`), always pre-calculate the required capacity before performing bulk append operations.

## 2026-06-10 - Avoid Committing Profiling/Test Artifacts
**Learning:** During performance analysis, running tests or profiling commands can generate artifact files (like `cpu.prof` or `tests.test`). Submitting these artifacts bloats the repository and fails git hygiene checks.
**Action:** Always ensure any test binaries or profiling output files are either removed or specifically ignored (via `.gitignore` or `git rm --cached`) before finalizing code changes and creating a PR.
