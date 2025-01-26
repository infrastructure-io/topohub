FROM ubuntu:24.10

# Install networking tools
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
    iproute2 \
    isc-dhcp-client \
    tcpdump \
    iputils-ping \
    curl \
    net-tools \
    && rm -rf /var/lib/apt/lists/*

# Set default command
CMD ["/bin/bash"]
