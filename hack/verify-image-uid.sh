#!/usr/bin/env bash
# Verify supported images still match the runtime identity hardcoded by the operator.

set -euo pipefail

EXPECTED_UID=999
EXPECTED_GID=999

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
