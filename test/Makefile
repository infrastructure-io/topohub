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
	docker network rm $(KIND_CLUSTER_NAME)-secondary || true
	docker network create --driver=bridge --subnet=$(KIND_NETWORK_SUBNET) --gateway=$(KIND_NETWORK_GATEWAY) $(KIND_CLUSTER_NAME)-secondary
	docker network connect --ip=$(KIND_CONTROL_IP) $(KIND_CLUSTER_NAME)-secondary $(KIND_CLUSTER_NAME)-control-plane
	docker network connect --ip=$(KIND_WORKER1_IP) $(KIND_CLUSTER_NAME)-secondary $(KIND_CLUSTER_NAME)-worker
	docker network connect --ip=$(KIND_WORKER2_IP) $(KIND_CLUSTER_NAME)-secondary $(KIND_CLUSTER_NAME)-worker2

installSpiderpool:
	@echo "Installing spiderpool..."
	chmod +x scripts/install-spiderpool.sh
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) ./scripts/install-spiderpool.sh

installDepsRedfish:
	@echo "Installing redfish..."
	chmod +x scripts/install-redfish-dhcp.sh
	chmod +x scripts/install-redfish-static.sh
	@echo "deploy a mock redifsh with dhcp ip "
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME)  UNDERLAY_CNI="spiderpool/eth1-macvlan" \
			NODE_NAME="$(KIND_CLUSTER_NAME)-control-plane" \
			./scripts/install-redfish-static.sh  ; ./scripts/hostendpoint.sh
	@echo "deploy a mock redifsh with statis ip "
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) UNDERLAY_CNI="spiderpool/eth1-macvlan" \
			HELM_NAME="vlan0-dhcp" NODE_NAME="$(KIND_CLUSTER_NAME)-control-plane" \
			./scripts/install-redfish-dhcp.sh
	@echo "deploy a mock redifsh with dhcp ip "
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) UNDERLAY_CNI="spiderpool/eth1-vlan10-macvlan" \
			HELM_NAME="vlan10-dhcp" NODE_NAME="$(KIND_CLUSTER_NAME)-control-plane" \
			./scripts/install-redfish-dhcp.sh
	@echo "deploy a bad redifsh with dhcp ip "
	E2E_CLUSTER_NAME=$(KIND_CLUSTER_NAME) UNDERLAY_CNI="spiderpool/eth1-vlan10-macvlan" \
			HELM_NAME="vlan10-invalid" NODE_NAME="$(KIND_CLUSTER_NAME)-control-plane" \
			DISABLE_REDFISH_MOCKUP="true" \
			./scripts/install-redfish-dhcp.sh

# Deploy application to kind cluster
deployTopohub:
	@echo "deploy topohub"
	chmod +x scripts/install-topohub.sh
	CLUSTER_NAME=$(KIND_CLUSTER_NAME) IMAGE_VERSION=$(VERSION) \
		PYROSCOPE_LOCAL_PORT=$(PYROSCOPE_LOCAL_PORT) \
		PYROSCOPE_CONTAINER_NAME=$(PYROSCOPE_CONTAINER_NAME) \
		IMAGE_PYROSCOPE_NAME=$(IMAGE_PYROSCOPE_NAME) \
		IMAGE_NAME=$(TOPOHUB_IMAGE_REF) ./scripts/install-topohub.sh
	chmod +x scripts/subnet.sh && ./scripts/subnet.sh


show:
	@echo "" ; echo "kubectl get hostendpoints -o wide" ; \
		kubectl get hostendpoints -o wide 
	@echo "" ; echo "kubectl get hostoperations -o wide" ; \
		kubectl get hostoperations -o wide
	@echo "" ; echo "kubectl get subnets -o wide" ; \
		kubectl get subnets -o wide
	@echo "" ; echo "kubectl get redfishstatuses -o wide" ; \
		kubectl get redfishstatuses -o wide
	@echo "" ; echo "kubectl get bindingips -o wide" ; \
		kubectl get bindingips -o wide
	@DEFAULT_INT=`ip r | grep default | head -1 | awk '{print $$5}' ` ; \
		INT_IPV4_ADDR=` ip a show $${DEFAULT_INT} | grep -oP '(?<=inet\s)[0-9]+(\.[0-9]+){3}' ` ; \
		echo "" ; \
		echo "visit the filebrowser: " ; \
		echo "     http://$${INT_IPV4_ADDR}:30080" ;\
		echo "     or " ; \
		echo "     http://$${INT_IPV4_ADDR}:30081" ;\
		echo "     username: admin" ;\
		echo "     password: admin"


# Clean up kind cluster
clean:
	@echo "Cleaning up kind cluster: $(KIND_CLUSTER_NAME)"
	@if kind get clusters | grep -q $(KIND_CLUSTER_NAME); then \
		kind delete cluster --name $(KIND_CLUSTER_NAME); \
		echo "Kind cluster deleted successfully"; \
	else \
		echo "Kind cluster $(KIND_CLUSTER_NAME) does not exist"; \
	fi
	docker stop $(PYROSCOPE_CONTAINER_NAME) || true
	docker rm $(PYROSCOPE_CONTAINER_NAME) || true
	docker network rm $(KIND_CLUSTER_NAME)-secondary || true
