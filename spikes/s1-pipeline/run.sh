#!/usr/bin/env bash
# Spike S1 (roadmap M1.1): reproduces every measurement in docs/adr/0002.
# Works on a temporary copy of ./app, so this directory is never modified.
set -u
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
cold=$(mktemp -d)
trap 'chmod -R u+w "$cold" 2>/dev/null; rm -rf "$work" "$cold"' EXIT
cp -r "$here/app" "$work/app"
cd "$work/app"
go mod tidy >/dev/null 2>&1 || { echo "go mod tidy failed"; exit 1; }
schema=example.com/app/internal/settings/schema
gen() { go run ./internal/fakegen "$@" "$schema"; }

echo "## $(go version)"
echo
echo "## 1. Placements (normal module mode, warm cache)"
for p in overlay overlay-file underscore dot dot-file plain; do gen -place=$p; done

echo
echo "## 2. Cold vs warm (fresh GOCACHE for the inner go run only)"
# Build the generator first, so building it doesn't warm the fresh cache.
go build -o "$work/fakegen" ./internal/fakegen
for p in overlay plain; do
  chmod -R u+w "$cold" 2>/dev/null; rm -rf "$cold"; mkdir -p "$cold"
  echo "-- $p, cold:"; GOCACHE=$cold "$work/fakegen" -place=$p "$schema"
  echo "-- $p, warm:"; GOCACHE=$cold "$work/fakegen" -place=$p "$schema"
  echo "-- $p, warm again:"; GOCACHE=$cold "$work/fakegen" -place=$p "$schema"
done

echo
echo "## 3. Workspace mode (go.work above the module)"
printf 'go 1.26.0\n\nuse ./app\n' > "$work/go.work"
for p in overlay overlay-file underscore plain; do gen -place=$p; done
rm "$work/go.work"

echo
echo "## 4. Vendor mode (-mod=vendor, with a third-party dependency vendored)"
go mod vendor
for p in overlay overlay-file underscore plain; do GOFLAGS=-mod=vendor gen -place=$p; done
rm -rf vendor

echo
echo "## 5. From inside go generate"
for p in overlay plain; do CFGX_PLACE=$p go generate ./internal/settings; done

echo
echo "## 6. Whole go generate step, warm (includes building the generator)"
CFGX_PLACE=overlay bash -c 'time go generate ./internal/settings' 2>&1 | grep -E 'place=|real'
