#!/usr/bin/env bash

# Refresh the vendored Vega-Lite browser runtime under internal/assets/rich/.
#
# Downloads the pinned releases as npm package artifacts (so the bytes always
# match what npm published, never a hand-grabbed CDN copy), extracts the
# browser builds plus each package's LICENSE, and drops them next to the other
# rich-content runtimes. The assets.go `//go:embed all:rich` directive picks
# them up with no code change; NOTICE.md must be updated by hand when a
# version constant below changes.

set -euo pipefail

VEGA_VERSION="6.4.0"
VEGA_LITE_VERSION="6.4.3"
VEGA_EMBED_VERSION="7.1.0"

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
rich_dir="${repo_root}/internal/assets/rich"

for tool in npm tar; do
	if ! command -v "${tool}" >/dev/null 2>&1; then
		echo "Error: ${tool} is required but not installed" >&2
		exit 2
	fi
done

pack_root=$(mktemp -d)
cleanup() {
	rm -rf -- "${pack_root}"
}
trap cleanup EXIT

cd "${pack_root}"
npm pack --silent "vega@${VEGA_VERSION}" "vega-lite@${VEGA_LITE_VERSION}" "vega-embed@${VEGA_EMBED_VERSION}"

# vendor <package-version> <tarball> <build file> <target base name>
vendor() {
	local version=$1 tarball=$2 build_file=$3 base=$4
	tar -xzf "${tarball}" "package/${build_file}" package/LICENSE
	cp "package/${build_file}" "${rich_dir}/${base}.min.js"
	cp package/LICENSE "${rich_dir}/LICENSE.${base}"
	echo "[vega-runtime] ${base} ${version} -> ${base}.min.js, LICENSE.${base}"
}

vendor "${VEGA_VERSION}" "vega-${VEGA_VERSION}.tgz" "build/vega.min.js" vega
vendor "${VEGA_LITE_VERSION}" "vega-lite-${VEGA_LITE_VERSION}.tgz" "build/vega-lite.min.js" vega-lite
vendor "${VEGA_EMBED_VERSION}" "vega-embed-${VEGA_EMBED_VERSION}.tgz" "build/vega-embed.min.js" vega-embed

echo "[vega-runtime] done; remember to update internal/assets/rich/NOTICE.md"
echo "[vega-runtime] and internal/export/page.go when a pinned version changes"
