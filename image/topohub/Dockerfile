# Build stage
FROM docker.io/library/golang:1.26.4 as builder

# Copy the entire project
WORKDIR /workspace
COPY . .

# Set GOPROXY for better download speed in China
ENV GOPROXY=https://goproxy.io

ARG TARGETARCH
ARG DISABLE_RACE

# Build the agent binary using the project's Makefile
RUN make build-topohub GOARCH=$TARGETARCH DISABLE_RACE=$DISABLE_RACE

# Final stage
FROM ubuntu:24.04

ARG GIT_COMMIT_VERSION
ENV ENV_GIT_COMMIT_VERSION=${GIT_COMMIT_VERSION}
ARG GIT_COMMIT_TIME
ENV ENV_GIT_COMMIT_TIMESTAMP=${GIT_COMMIT_TIME}
ARG PROJECT_VERSION
ENV ENV_PROJECT_VERSION=${PROJECT_VERSION}

# Install required packages including ipmitool
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
    dnsmasq \
    iproute2 \
    isc-dhcp-client \
    vim \
    ipmitool \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /
# Copy the binary from builder stage
COPY --from=builder /workspace/bin/topohub /usr/local/bin/
COPY ./image/topohub/files /files
COPY ./ostools  /tools

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/topohub"]
