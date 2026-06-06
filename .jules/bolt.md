## 2024-06-06 - Optimized CacheMiddleware Key Generation
**Learning:** In Go, converting a `sha256` hash to a hex string to use as a map key in a hot path causes significant heap allocation overhead (for the interface, byte slice, and string). Using the `[32]byte` output of `sha256.Sum256()` directly as the map key eliminates these allocations and runs significantly faster.
**Action:** When implementing caching mechanisms or any logic requiring hashes as identifiers in hot paths, avoid `hex.EncodeToString` and prefer using fixed-size byte arrays directly as map keys where applicable.
