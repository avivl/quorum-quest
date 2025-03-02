#!/bin/bash
# symlink_proto.sh - Creates a symlink to the proto files instead of using Python package structure

# Get the absolute path of the script and the repository root directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "Repository root: ${REPO_ROOT}"

# Create quorum_quest_client/proto directory
PROTO_DIR="${SCRIPT_DIR}/quorum_quest_client/proto"
mkdir -p "${PROTO_DIR}"
touch "${PROTO_DIR}/__init__.py"

# Copy proto files to client directory (more reliable than symlinks on some systems)
echo "Copying proto files to ${PROTO_DIR}..."
cp "${REPO_ROOT}/api/gen/python/v1/quorum_quest_api_pb2.py" "${PROTO_DIR}/"
cp "${REPO_ROOT}/api/gen/python/v1/quorum_quest_api_pb2_grpc.py" "${PROTO_DIR}/"
touch "${PROTO_DIR}/quorum_quest_api_pb2.py"
touch "${PROTO_DIR}/quorum_quest_api_pb2_grpc.py"

echo "Proto files copied successfully"