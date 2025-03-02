#!/bin/bash
# setup-all.sh - Combined script that performs all setup steps

# Get the absolute path of the script and the repository root directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "Repository root: ${REPO_ROOT}"

# Create directories
mkdir -p "${SCRIPT_DIR}/quorum_quest_client/proto"

# Create missing __init__.py files
touch "${SCRIPT_DIR}/quorum_quest_client/proto/__init__.py"

# Copy proto files to client directory
echo "Copying proto files to ${SCRIPT_DIR}/quorum_quest_client/proto..."
cp "${REPO_ROOT}/api/gen/python/v1/quorum_quest_api_pb2.py" "${SCRIPT_DIR}/quorum_quest_client/proto/"
cp "${REPO_ROOT}/api/gen/python/v1/quorum_quest_api_pb2_grpc.py" "${SCRIPT_DIR}/quorum_quest_client/proto/"

echo "Proto files copied successfully"

# Clean up any existing egg-info directories to avoid conflicts
find "${SCRIPT_DIR}" -name "*.egg-info" -type d -exec rm -rf {} + 2>/dev/null || true

# Install the client package in development mode
cd "${SCRIPT_DIR}"
echo "Installing from directory: $(pwd)"
pip install -e .

echo "Installed quorum-quest-client in development mode"
echo "Done! You can now run the example:"
echo "python ${SCRIPT_DIR}/example/main.py"