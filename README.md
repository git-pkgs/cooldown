# cooldown

A small, ecosystem-agnostic version-age filter for package-manager tools. Hides versions published too recently so the community has time to spot malicious releases before they're pulled into projects.

Cross-ecosystem by construction: the same `Config` shape covers npm, PyPI, Cargo, RubyGems, Composer, Conda, Hex, NuGet, Pub, and any future ecosystem. Resolution order is package-PURL > ecosystem-name > global default, so a single config can express a strict default with targeted opt-outs.

## Install

```
go get github.com/git-pkgs/cooldown
```

## Usage

```go
cfg := &cooldown.Config{
    Default:    "48h",                              // global window
    Ecosystems: map[string]string{"npm": "72h"},    // npm gets longer
    Packages:   map[string]string{                  // per-PURL override
        "pkg:npm/htmx.org": "0",                    // 0 = disabled
    },
}

if cfg.IsAllowed("npm", "pkg:npm/lodash", publishedAt) {
    // version cleared the window; use it
}
```

`Config.For(ecosystem, purl)` returns the effective duration; useful when surfacing the policy to a UI. `Config.Enabled()` reports whether any cooldown is configured (cheap check before walking a large version set).

Duration strings accept Go's standard formats (`48h`, `30m`, `1h30m`) plus a `d` suffix for days (`3d`). `0` disables the window.

## Why standalone

Originally lived inside an HTTP proxy that filtered registry responses; spotted as a reusable shape and lifted out. The same predicate is useful for package-manager CLIs, dependency-update bots, security scanners, and anything else that walks a version set with an age constraint.

## License

MIT
