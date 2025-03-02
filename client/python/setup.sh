#!/bin/bash
# setup.sh - Creates necessary __init__.py files for the Python package structure

# Get the absolute path of the script and the repository root directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "Repository root: ${REPO_ROOT}"

# Create missing __init__.py files
mkdir -p "${REPO_ROOT}/api/gen/python/v1"

touch "${REPO_ROOT}/api/__init__.py"
touch "${REPO_ROOT}/api/gen/__init__.py"
touch "${REPO_ROOT}/api/gen/python/__init__.py"
touch "${REPO_ROOT}/api/gen/python/v1/__init__.py"

echo "Created package structure with __init__.py files"

# Clean up any existing egg-info directories to avoid conflicts
find "${SCRIPT_DIR}" -name "*.egg-info" -type d -exec rm -rf {} + 2>/dev/null || true

# Install the client package in development mode
cd "${SCRIPT_DIR}"
echo "Installing from directory: $(pwd)"
pip install -e .

echo "Installed quorum-quest-client in development mode"
echo "Done! You can now run the example:"
echo "python ${SCRIPT_DIR}/example/main.py"