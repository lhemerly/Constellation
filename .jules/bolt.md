## 2024-04-21 - Optimize GRPCConnection.IsConnected()
**Learning:** Checking the connection state using `sync.Mutex` caused contention when `IsConnected()` was called repeatedly (e.g., in Send/Receive loops under high concurrency). The benchmark showed ~90ns/op with `sync.Mutex` overhead.
**Action:** Replaced `sync.Mutex` lock/unlock in `IsConnected()` with an `atomic.Bool` (`isConnected.Load()`). Benchmark improved to ~0.67ns/op. Ensure that any future state checks in high-frequency hot paths utilize atomic operations when possible to avoid lock contention.
