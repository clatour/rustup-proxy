## Simple Rustup Proxy

This repository implements a fairly simple pass-through caching proxy for [rustup](https://github.com/rust-lang/rustup)

```
# Usage
CACHE_PATH=./cache HOST=http://127.0.0.1:8080 go run main.go
RUSTUP_DIST_SERVER=http://127.0.0.1:8080 rustup -v update
```

URLs in the rustup manifests are rewritten to the given `$HOST`, and new shas are calculated on the fly

First thoughts for todos:
- Smarter manifest caching, currently passes through every time.
- Some sort of LRU pruning
