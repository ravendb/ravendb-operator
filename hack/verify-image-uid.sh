#!/usr/bin/env bash
#
# Guards the operator's hardcoded RavenDB runtime identity against upstream drift.
#
# The operator sets the pod fsGroup and container runAsUser/runAsGroup to a fixed
# 999 (pkg/common.RavenDBUID / RavenDBGID). Kubernetes cannot infer that value
# from the image, so we restate it in code. If a RavenDB image ever ships a
# different USER, that hardcoded 999 would silently break DataDir permissions on
# a user's cluster. This check inspects the actual image(s) so the drift fails
# in CI instead.
#
# Usage: hack/verify-image-uid.sh [image ...]   (defaults to ravendb/ravendb:latest)

set -euo pipefail

EXPECTED_UID=999
EXPECTED_GID=999

images=("$@")
if [ ${#images[@]} -eq 0 ]; then
  images=("ravendb/ravendb:latest")
fi

fail=0
for img in "${images[@]}"; do
  echo ">> inspecting ${img}"
  docker pull -q "${img}" >/dev/null

  user=$(docker image inspect --format '{{.Config.User}}' "${img}")
  id_out=$(docker run --rm --entrypoint id "${img}")
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
