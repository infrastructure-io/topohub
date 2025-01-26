# Include common definitions
include ../Makefile.def

.PHONY: init installDeps deploy installDepsRedfish clean

# Kind cluster configuration
KIND_CLUSTER_NAME ?= bmc-e2e
KIND_CONFIG = kind-config.yaml

# Initialize kind cluster with 2 nodes
init:
	@echo "Creating kind cluster with name: $(KIND_CLUSTER_NAME)"
	kind create cluster --name $(KIND_CLUSTER_NAME) --config $(KIND_CONFIG)
	@echo "Kind cluster created successfully"
	@echo "storageclass local-path does not allow pods with a specified NodeName, so it makes all pods to run on the worker node"
	kubectl taint node $(KIND_CLUSTER_NAME)-control-plane local-path=nopod:NoSchedule


installDeps:
	@echo "Installing spiderpool..."
	chmod +x scripts/install-spiderpool.sh
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) ./scripts/install-spiderpool.sh

installDepsRedfish:
	@echo "Installing redfish..."
	chmod +x scripts/install-redfish-dhcp.sh
	chmod +x scripts/install-redfish-static.sh
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) UNDERLAY_CNI="spiderpool/eth0-macvlan" \
			./scripts/install-redfish-dhcp.sh
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME)  UNDERLAY_CNI="spiderpool/eth0-macvlan" \
			STATIC_IP="192.168.0.50/24"  \
			./scripts/install-redfish-static.sh
	./scripts/hostendpoint.sh

# Deploy application to kind cluster
deploy:
	@echo "Loading images into kind cluster..."
	docker inspect $(CONTROLLER_IMAGE_REF) &>/dev/null || \
		docker pull $(CONTROLLER_IMAGE_REF)
	docker inspect $(AGENT_IMAGE_REF) &>/dev/null || \
		docker pull $(AGENT_IMAGE_REF)
	kind load docker-image $(CONTROLLER_IMAGE_REF) --name $(KIND_CLUSTER_NAME) && \
	kind load docker-image $(AGENT_IMAGE_REF) --name $(KIND_CLUSTER_NAME)
	@echo "Images loaded successfully"
	@echo "Deploying application using Helm chart..."
	- helm uninstall bmc -n bmc --wait &>/dev/null || true
	@echo "for macvlan deployment"
	helm install bmc ../chart \
		--namespace bmc \
		--create-namespace \
		--wait \
		--set logLevel="debug" \
		--set image.tag=$(VERSION) \
		--set clusterAgent.agentYaml.image.tag=$(VERSION) \
		--set clusterAgent.endpoint.https=false \
		--set clusterAgent.agentYaml.hostNetwork=false \
		--set clusterAgent.agentYaml.underlayInterface="spiderpool/eth0-macvlan" \
		--set clusterAgent.feature.logLevel="debug" \
		--set clusterAgent.feature.dhcpServerConfig.dhcpServerInterface="net1" \
		--set clusterAgent.feature.dhcpServerConfig.subnet="192.168.0.0/24" \
		--set clusterAgent.feature.dhcpServerConfig.ipRange="192.168.0.100-192.168.0.200" \
		--set clusterAgent.feature.dhcpServerConfig.gateway="192.168.0.1" \
		--set clusterAgent.feature.dhcpServerConfig.selfIp="192.168.0.2/24" \
		--set clusterAgent.endpoint.username="" \
		--set clusterAgent.endpoint.password="" \
		--set clusterAgent.endpoint.port=8000 \
		--set clusterAgent.storage.type="pvc"

	# @echo "for host network deployment" 	
	# helm install bmc ../chart \
	# 	--namespace bmc \
	# 	--create-namespace \
	# 	--wait \
	# 	--set image.repository=$(CONTROLLER_IMAGE) \
	# 	--set image.tag=$(VERSION) \
	# 	--set clusterAgent.agentYaml.image.repository=$(AGENT_IMAGE) \
	# 	--set clusterAgent.agentYaml.image.tag=$(VERSION) \
	# 	--set clusterAgent.endpoint.https=false \
	# 	--set clusterAgent.agentYaml.nodeName="$(KIND_CLUSTER_NAME)-worker" \
	# 	--set clusterAgent.agentYaml.hostNetwork=true \
	# 	--set clusterAgent.agentYaml.underlayInterface="" \
	# 	--set clusterAgent.feature.dhcpServerConfig.dhcpServerInterface="eth0" \
	# 	--set clusterAgent.feature.dhcpServerConfig.selfIp="" \
	# 	--set clusterAgent.feature.dhcpServerConfig.subnet="172.18.0.0/16" \
	# 	--set clusterAgent.feature.dhcpServerConfig.ipRange="172.18.0.100-172.18.0.200" \
	# 	--set clusterAgent.feature.dhcpServerConfig.gateway="172.18.0.10" \
	# 	--set clusterAgent.endpoint.username="test" \
	# 	--set clusterAgent.endpoint.password="abc" \
	# 	--set clusterAgent.endpoint.port=8000

# Clean up kind cluster
clean:
	@echo "Cleaning up kind cluster: $(KIND_CLUSTER_NAME)"
	@if kind get clusters | grep -q $(KIND_CLUSTER_NAME); then \
		kind delete cluster --name $(KIND_CLUSTER_NAME); \
		echo "Kind cluster deleted successfully"; \
	else \
		echo "Kind cluster $(KIND_CLUSTER_NAME) does not exist"; \
	fi
