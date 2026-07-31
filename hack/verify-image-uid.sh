#!/usr/bin/env bash
# Verify supported images still match the runtime identity the operator sets,
# read straight from pkg/common/constants.go so both sides cannot drift apart.

set -euo pipefail

CONSTANTS_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/pkg/common/constants.go"

read_id_constant() {
  sed -n "s/^[[:space:]]*$1[[:space:]][[:space:]]*int64[[:space:]]*=[[:space:]]*\([0-9][0-9]*\).*/\1/p" "${CONSTANTS_FILE}"
}

EXPECTED_UID=$(read_id_constant RavenDBUID)
EXPECTED_GID=$(read_id_constant RavenDBGID)
if [ -z "${EXPECTED_UID}" ] || [ -z "${EXPECTED_GID}" ]; then
  echo "ERROR: cannot read RavenDBUID/RavenDBGID from ${CONSTANTS_FILE}" >&2
  exit 1
fi

images=("$@")
if [ ${#images[@]} -eq 0 ]; then
  images=(
    "ravendb/ravendb:6.2.11-ubuntu.22.04-x64"
    "ravendb/ravendb:7.1.3-ubuntu.22.04-x64"
  )
fi

fail=0
for img in "${images[@]}"; do
  echo ">> inspecting ${img}"
  docker pull -q "${img}" >/dev/null

  user=$(docker image inspect --format '{{.Config.User}}' "${img}")
  id_out=$(docker run --rm \
    --network=none \
    --read-only \
    --cap-drop=ALL \
    --security-opt=no-new-privileges \
    --entrypoint id \
    "${img}")
  echo "   Config.User=${user:-<empty>}  ${id_out}"

  if ! echo "${id_out}" | grep -q "uid=${EXPECTED_UID}("; then
    echo "   ERROR: uid drift in ${img} (expected ${EXPECTED_UID}); update pkg/common.RavenDBUID" >&2
    fail=1
  fi
  if ! echo "${id_out}" | grep -q "gid=${EXPECTED_GID}("; then
    echo "   ERROR: gid drift in ${img} (expected ${EXPECTED_GID}); update pkg/common.RavenDBGID" >&2
    fail=1
  fi
done

if [ "${fail}" -ne 0 ]; then
  echo "image UID/GID drift detected" >&2
  exit 1
fi

echo "OK: image UID/GID matches pkg/common.RavenDBUID/RavenDBGID (${EXPECTED_UID}/${EXPECTED_GID})"
