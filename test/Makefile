# Include common definitions
include ../Makefile.def

.PHONY: init installSpiderpool deployTopohub installDepsRedfish clean show

# Kind cluster configuration
KIND_CLUSTER_NAME ?= topohub
KIND_CONFIG = kind-config.yaml

# Initialize kind cluster with 2 nodes
init:
	@echo "Creating kind cluster with name: $(KIND_CLUSTER_NAME)"
	kind create cluster --name $(KIND_CLUSTER_NAME) --config $(KIND_CONFIG)
	@echo "Kind cluster created successfully"
	@echo "storageclass local-path does not allow pods with a specified NodeName, so it makes all pods to run on the worker node"
	kubectl taint node $(KIND_CLUSTER_NAME)-control-plane local-path=nopod:NoSchedule


installSpiderpool:
	@echo "Installing spiderpool..."
	chmod +x scripts/install-spiderpool.sh
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) ./scripts/install-spiderpool.sh

installDepsRedfish:
	@echo "Installing redfish..."
	chmod +x scripts/install-redfish-dhcp.sh
	chmod +x scripts/install-redfish-static.sh
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME)  UNDERLAY_CNI="spiderpool/eth0-macvlan" NODE_NAME="$(KIND_CLUSTER_NAME)-control-plane" \
			./scripts/install-redfish-static.sh  ; ./scripts/hostendpoint.sh
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) UNDERLAY_CNI="spiderpool/eth0-macvlan" NODE_NAME="$(KIND_CLUSTER_NAME)-control-plane" \
			./scripts/install-redfish-dhcp.sh

# Deploy application to kind cluster
deployTopohub:
	@echo "deploy topohub"
	chmod +x scripts/install-topohub.sh
	CLUSTER_NAME=$(KIND_CLUSTER_NAME) IMAGE_VERSION=$(VERSION) IMAGE_NAME=$(TOPOHUB_IMAGE_REF) ./scripts/install-topohub.sh
	
	# @echo "for host network deployment" 	
	# helm install topohub ../chart \
	# 	--namespace topohub \
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

show:
	@echo "" ; echo "kubectl get hostendpoints -o wide" ; \
		kubectl get hostendpoints -o wide 
	@echo "" ; echo "kubectl get hostoperations -o wide" ; \
		kubectl get hostoperations -o wide
	@echo "" ; echo "kubectl get subnets -o wide" ; \
		kubectl get subnets -o wide
	@echo "" ; echo "kubectl get hoststatuses -o wide" ; \
		kubectl get hoststatuses -o wide

# Clean up kind cluster
clean:
	@echo "Cleaning up kind cluster: $(KIND_CLUSTER_NAME)"
	@if kind get clusters | grep -q $(KIND_CLUSTER_NAME); then \
		kind delete cluster --name $(KIND_CLUSTER_NAME); \
		echo "Kind cluster deleted successfully"; \
	else \
		echo "Kind cluster $(KIND_CLUSTER_NAME) does not exist"; \
	fi
