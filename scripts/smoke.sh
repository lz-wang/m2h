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
printf '# Clean\n' > clean.md

actual_version=$("${binary}" --version)
if [[ "${actual_version}" != "${expected_version}" ]]; then
	echo "Error: version output \"${actual_version}\", expected \"${expected_version}\"" >&2
	exit 1
fi
echo "[smoke] version ${actual_version}"

"${binary}" export smoke.md --output smoke.html --mode dark
grep -F '<title>Smoke</title>' smoke.html >/dev/null
grep -F 'href="next.md"' smoke.html >/dev/null
echo "[smoke] export"

"${binary}" check clean.md > check.log
grep -F 'Checked 1 Markdown file: no issues found' check.log >/dev/null
"${binary}" check clean.md --format json | grep -F '"files": 1' >/dev/null
if "${binary}" check smoke.md > /dev/null 2>&1; then
	echo "Error: check accepted a not-served sibling Markdown target" >&2
	exit 1
fi

# The rule engine must reach the release binary: a default warning, the
# --strict exit code, and an opt-in rule behind --enable.
printf '# Title\n\n### Skip\n' > lint.md
"${binary}" check lint.md > lint.log
grep -F 'warning [heading.level-skip]' lint.log >/dev/null
grep -F 'Checked 1 Markdown file: 1 warning' lint.log >/dev/null
if "${binary}" check lint.md --strict > /dev/null 2>&1; then
	echo "Error: check --strict did not fail on a warning" >&2
	exit 1
fi

printf '# Title\n\n## Empty\n\n## Next\n\ncontent\n' > empty-section.md
"${binary}" check empty-section.md > empty-section.log
grep -F 'Checked 1 Markdown file: no issues found' empty-section.log >/dev/null
"${binary}" check empty-section.md --enable section.empty > opt-in.log
grep -F 'warning [section.empty]' opt-in.log >/dev/null
echo "[smoke] check"

port=${M2H_SMOKE_PORT:-18793}
"${binary}" smoke.md --host 127.0.0.1 --port "${port}" --no-open > preview.log 2>&1 &
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
	echo "Error: web server did not become ready" >&2
	cat preview.log >&2
	exit 1
fi
grep -F '<title>m2h</title>' preview.html >/dev/null
curl --fail --silent "http://127.0.0.1:${port}/api/document?path=smoke.md" > document.json
grep -F '"title":"Smoke"' document.json >/dev/null
grep -F '"text":"Smoke"' document.json >/dev/null
echo "[smoke] web"
