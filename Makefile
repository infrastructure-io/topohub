# Include common definitions
include Makefile.def

# Default target
.PHONY: all
all: images

# Build targets
.PHONY: build-binaries
build-binaries: build-topohub

.PHONY: build-topohub
build-topohub:
	$(GO_BUILD) -o $(BIN_DIR)/topohub cmd/topohub/main.go

# Image targets
.PHONY: images
images: topohub-image tools-image

.PHONY: topohub-image
topohub-image:
	docker build $(DOCKER_BUILD_FLAGS) -t $(TOPOHUB_IMAGE_REF) -f image/topohub/Dockerfile .

.PHONY: tools-image
tools-image: build-tools-image

.PHONY: build-tools-image
build-tools-image:
	docker build -t $(TOOLS_IMAGE_REF) -f image/tools/Dockerfile image/tools

# Helm chart
#================== chart
ROOT_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
CHART_DIR := $(ROOT_DIR)/chart
DESTDIR_CHART ?= $(ROOT_DIR)/output/chart

.PHONY: chart_package
chart_package: lint_chart_format lint_chart_version
	-@rm -rf $(DESTDIR_CHART)
	-@mkdir -p $(DESTDIR_CHART)
	cd $(DESTDIR_CHART) ; \
   		echo "package chart " ; \
   		helm package  $(CHART_DIR) ; \


.PHONY: update_chart_version
update_chart_version:
	VERSION=`cat VERSION | tr -d '\n' ` ; [ -n "$${VERSION}" ] || { echo "error, wrong version" ; exit 1 ; } ; \
		echo "update chart version to $${VERSION}" ; \
		CHART_VERSION=`echo $${VERSION} | tr -d 'v' ` ; \
		sed -E -i 's?^version: .*?version: '$${CHART_VERSION}'?g' $(CHART_DIR)/Chart.yaml &>/dev/null  ; \
		sed -E -i 's?^appVersion: .*?appVersion: "'$${CHART_VERSION}'"?g' $(CHART_DIR)/Chart.yaml &>/dev/null  ; \
   		echo "version of all chart is right"


.PHONY: lint_chart_format
lint_chart_format:
	mkdir -p $(DESTDIR_CHART) ; \
   			echo "check chart" ; \
   			helm lint --with-subcharts $(CHART_DIR)


.PHONY: lint_chart_version
lint_chart_version:
	VERSION=`cat VERSION | tr -d '\n' ` ; [ -n "$${VERSION}" ] || { echo "error, wrong version" ; exit 1 ; } ; \
		echo "check chart version $${VERSION}" ; \
		CHART_VERSION=`echo $${VERSION} | tr -d 'v' ` ; \
			grep -E "^version: $${CHART_VERSION}" $(CHART_DIR)/Chart.yaml &>/dev/null || { echo "error, wrong version in Chart.yaml" ; exit 1 ; } ; \
			grep -E "^appVersion: \"$${CHART_VERSION}\"" $(CHART_DIR)/Chart.yaml &>/dev/null || { echo "error, wrong appVersion in Chart.yaml" ; exit 1 ; } ; \
   		echo "version of all chart is right"


#================= update golang

GO_VERSION := $(shell cat GO_VERSION | tr -d '\n' )
GO_IMAGE_VERSION = $(shell echo ${GO_VERSION} | awk -F. '{ z=$$3; if (z == "") z=0; print $$1 "." $$2 "." z}' )
#GO_MAJOR_AND_MINOR_VERSION = $(shell  echo "${GO_VERSION}" | grep  -o -E '^[0-9]+\.[0-9]+' )


## Update Go version for all the components
.PHONY: update_go_version
update_go_version: update_images_dockerfile_golang update_mod_golang update_workflow_golang

.PHONY: update_images_dockerfile_golang
update_images_dockerfile_golang:
	echo "update images dockerfile golang to $(GO_VERSION)"
	GO_VERSION=$(GO_VERSION) $(ROOT_DIR)/tools/scripts/update-golang-image.sh

# Update Go version for GitHub workflow
.PHONY: update_workflow_golang
update_workflow_golang:
		echo "update workflow golang to ${GO_IMAGE_VERSION}" ; \
		for fl in $(shell find .github/workflows -name "*.yaml" -print) ; do \
  			sed -i 's/go-version: .*/go-version: '${GO_IMAGE_VERSION}'/g' $$fl ; \
  			done

# Update Go version in go.mod
.PHONY: update_mod_golang
update_mod_golang:
		echo "update go.mod to ${GO_VERSION}" ; \
		sed -i -E 's/^go .*/go '"${GO_VERSION}"'/g' go.mod


#-------------------------------------------
# E2E tests
.PHONY: e2e e2e-clean uninstall_e2e

# Run E2E tests
e2e:
	@echo "Setting up E2E test environment..."
	$(MAKE) -C test clean
	$(MAKE) -C test init
	$(MAKE) -C test deployTopohub
	$(MAKE) -C test installSpiderpool
	$(MAKE) -C test installDepsRedfish
	@echo "E2E environment setup completed"

# Clean up E2E environment
e2e-clean:
	@echo "Cleaning up E2E environment..."
	$(MAKE) -C test clean
	@echo "E2E environment cleanup completed"

# Uninstall E2E environment
uninstall_e2e:
	@echo "Uninstalling E2E environment..."
	$(MAKE) -C test kind-delete
	@echo "E2E environment uninstalled successfully"

# Cleanup
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
	rm -f *.tgz

.PHONY: update_crd_sdk
update_crd_sdk:
	@ echo "update crd manifest" && ./tools/golang/crdControllerGen.sh
	@ echo "update crd sdk" && ./tools/golang/crdSdkGen.sh


.PHONY: validate_crd_sdk
validate_crd_sdk:
	@ echo "validate crd manifest"
	make update_crd_sdk ; \
		if ! test -z "$$(git status --porcelain)"; then \
  			echo "please run 'make update_crd_sdk' to update crd code" ; \
  			exit 1 ; \
  		fi ; echo "succeed to check crd sdk"


# Help
.PHONY: usage
usage:
	@echo "Available targets:"
	@echo "  all             - Build binaries, container images, and Helm chart"
	@echo "  build-binaries  - Build controller and agent binaries"
	@echo "  build-topohub   - Build topohub binary"
	@echo "  images          - Build container images"
	@echo "  topohub-image   - Build topohub container image"
	@echo "  tools-image     - Build tools container image"
	@echo "  chart           - Package Helm chart"
	@echo "  e2e             - Run E2E tests"
	@echo "  e2e-clean       - Clean up E2E environment"
	@echo "  uninstall_e2e  - Uninstall E2E environment"
	@echo "  clean           - Remove build artifacts"
	@echo "  usage           - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION        - Version tag for images (default: from git commit hash)"
	@echo "  REGISTRY       - Container image registry (default: ghcr.io/infrastructure-io)"
	@echo "  IMAGE_NAMESPACE - Container image namespace for tools (default: topohub)"
	@echo "  TOOLS_IMAGE_NAME - Name of the tools image (default: tools)"
	@echo "  TOOLS_IMAGE_TAG - Tag of the tools image (default: latest)"
	@echo "  GOOS          - Target OS for build (default: linux)"
	@echo "  GOARCH        - Target architecture for build (default: amd64)"
	@echo "  CGO_ENABLED   - Enable CGO for build (default: 0)"
