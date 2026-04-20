## 2024-08-01 - [IsConnected Hot Path Optimization]
**Learning:** Checking the connection state (IsConnected) using a sync.Mutex in hot paths (like Send/Receive) causes severe lock contention and affects concurrency throughput.
**Action:** Use atomic operations (e.g., `atomic.Bool`) instead of `sync.Mutex` for state checks in high-frequency hot paths to improve concurrency.
