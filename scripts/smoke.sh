#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 2 ]]; then
	echo "Usage: scripts/smoke.sh <binary> <expected-version>" >&2
	exit 2
fi

binary_input=$1
expected_version=$2
if [[ ! -f "${binary_input}" ]]; then
	echo "Error: smoke binary does not exist: ${binary_input}" >&2
	exit 2
fi

binary_dir=$(cd "$(dirname "${binary_input}")" && pwd -P)
binary="${binary_dir}/$(basename "${binary_input}")"
smoke_root=$(mktemp -d)
server_pid=""

cleanup() {
	if [[ -n "${server_pid}" ]]; then
		if [[ "${RUNNER_OS:-}" == "Windows" ]]; then
			taskkill.exe //IM "$(basename "${binary}")" //T //F >/dev/null 2>&1 || true
		else
			kill -TERM "${server_pid}" >/dev/null 2>&1 || true
			wait "${server_pid}" >/dev/null 2>&1 || true
		fi
	fi
	if [[ -n "${smoke_root}" && -d "${smoke_root}" ]]; then
		rm -rf -- "${smoke_root}"
	fi
}
trap cleanup EXIT

cd "${smoke_root}"
printf '# Smoke\n\n- %s\n\n[Next](next.md)\n' "${RUNNER_OS:-local}" > smoke.md
printf '# Next\n' > next.md

actual_version=$("${binary}" version)
if [[ "${actual_version}" != "${expected_version}" ]]; then
	echo "Error: version output \"${actual_version}\", expected \"${expected_version}\"" >&2
	exit 1
fi
echo "[smoke] version ${actual_version}"

"${binary}" convert smoke.md --output smoke.html --mode dark
grep -F '<title>Smoke</title>' smoke.html >/dev/null
grep -F 'href="next.html"' smoke.html >/dev/null
echo "[smoke] convert"

port=${M2H_SMOKE_PORT:-18793}
"${binary}" preview smoke.md --host 127.0.0.1 --port "${port}" > preview.log 2>&1 &
server_pid=$!
preview_ready=false
for _ in {1..30}; do
	if curl --fail --silent "http://127.0.0.1:${port}/" > preview.html; then
		preview_ready=true
		break
	fi
	sleep 1
done
if [[ "${preview_ready}" != "true" ]]; then
	echo "Error: preview server did not become ready" >&2
	cat preview.log >&2
	exit 1
fi
grep -F '<title>Smoke</title>' preview.html >/dev/null
grep -F 'class="m2h-mode-auto"' preview.html >/dev/null
echo "[smoke] preview"
