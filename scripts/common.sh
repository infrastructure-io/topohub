#!/bin/bash

# Registry configuration
REGISTRY=${REGISTRY:-"ghcr.io/infrastructure-io"}

# Image names
CONTROLLER_IMAGE_NAME="bmc-controller"
AGENT_IMAGE_NAME="bmc-agent"
TOOLS_IMAGE_NAME="bmc-tools"

# Image tags
VERSION=${VERSION:-$(git rev-parse --short HEAD)}
TOOLS_IMAGE_TAG=${TOOLS_IMAGE_TAG:-"latest"}

# Full image paths
CONTROLLER_IMAGE="${REGISTRY}/${CONTROLLER_IMAGE_NAME}"
AGENT_IMAGE="${REGISTRY}/${AGENT_IMAGE_NAME}"
TOOLS_IMAGE="${REGISTRY}/${TOOLS_IMAGE_NAME}"

# Full image references with tags
CONTROLLER_IMAGE_REF="${CONTROLLER_IMAGE}:${VERSION}"
AGENT_IMAGE_REF="${AGENT_IMAGE}:${VERSION}"
TOOLS_IMAGE_REF="${TOOLS_IMAGE}:${TOOLS_IMAGE_TAG}"
