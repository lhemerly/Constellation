## Performance Insights & Anti-Patterns

1. **Avoid Mutex Lock Contention on High-Frequency Hot Paths:**
   When dealing with high-frequency calls like checking connection status in `Send` and `Receive` loops within a `GRPCConnection`, replace `sync.Mutex` with atomic operations (like `atomic.Bool` from `sync/atomic`). This significantly eliminates lock contention and boosts overall concurrency throughput without sacrificing safety.

2. **Beware of "Close of Closed Channel" panics during concurrent operations:**
   In network or generic connection components where disconnect/reconnect logic exists, be extremely careful about closing channels that act as data pipes (e.g. `dataChan`). If a connection is subjected to concurrent uses or multiple disconnections, closing a channel that is already closed will trigger a panic. Simply dereferencing it or relying on connection states via an atomic variable is often safer.

3. **Pre-compile Middleware Chains instead of recompiling dynamically:**
   When using a Middleware pattern to wrap the core processing function (e.g., `BaseNode`), avoid recompiling or reconstructing the middleware chain repeatedly on every single `.Process()` invocation. Instead, pre-compile and cache the final wrapped function to avoid intense memory allocation and garbage collection (GC) pressure.

4. **Beware of Lock Inversion Deadlocks when extending nodes:**
   When a node struct (like `RouterNode`) extends another node (like `BaseNode`) via embedding, pay close attention to nested method calls. Never hold a lock on the extending struct's mutex (e.g., `routesMutex`) while calling base methods (e.g., `Subscribe`, `Unsubscribe`, `Process`) if those base methods internally acquire the base node's mutex. Release the extending node's lock first before invoking the base node to prevent deadlocks.
## 2024-04-21 - Optimize GRPCConnection.IsConnected()
**Learning:** Checking the connection state using `sync.Mutex` caused contention when `IsConnected()` was called repeatedly (e.g., in Send/Receive loops under high concurrency). The benchmark showed ~90ns/op with `sync.Mutex` overhead.
**Action:** Replaced `sync.Mutex` lock/unlock in `IsConnected()` with an `atomic.Bool` (`isConnected.Load()`). Benchmark improved to ~0.67ns/op. Ensure that any future state checks in high-frequency hot paths utilize atomic operations when possible to avoid lock contention.
## 2024-05-18 - Optimize Lock Contention in Notify Method
**Learning:** Holding a read lock (`RLock()`) for the entire duration of iterating over a map and spawning long-running or blocking goroutines (`wg.Wait()`) creates massive lock contention and opens the door for deadlocks. If any of the spawned workers attempt to acquire a write lock (`Lock()`) on the same mutex (e.g., trying to subscribe or unsubscribe), it will deadlock because the read lock is still held by the waiting parent.
**Action:** Always copy map/slice contents to a local slice under the lock, then immediately release the lock before iterating and processing the items concurrently. Apply this specifically when dealing with publisher/subscriber patterns in the codebase to keep the hot path lock-free.
## 2024-08-09 - CacheMiddleware Optimization
**Learning:** In performance-critical paths (e.g., `CacheMiddleware`), using `sha256.Sum256(input)` directly and using the fixed-size array `[32]byte` as a map key is significantly more efficient than instantiating `sha256.New()` and encoding strings with `hex.EncodeToString()`. This eliminates unnecessary heap allocations.
**Action:** Always prefer using fixed-size arrays returned by hash functions (like `[32]byte` from `sha256.Sum256`) as map keys instead of converting them to strings via hex encoding when optimizing for memory allocation.
