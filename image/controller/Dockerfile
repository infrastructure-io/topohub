# Build stage
FROM docker.io/library/golang:1.23.5 as builder

# Copy the entire project
WORKDIR /workspace
COPY . .

# Set GOPROXY for better download speed in China
ENV GOPROXY=https://goproxy.io

# Build the controller binary using the project's Makefile
RUN make build-controller

# Final stage
FROM ubuntu:24.10

ARG GIT_COMMIT_VERSION
ENV ENV_GIT_COMMIT_VERSION=${GIT_COMMIT_VERSION}
ARG GIT_COMMIT_TIME
ENV ENV_GIT_COMMIT_TIMESTAMP=${GIT_COMMIT_TIME}
ARG PROJECT_VERSION
ENV ENV_PROJECT_VERSION=${PROJECT_VERSION}

WORKDIR /
# Copy the binary from builder stage
COPY --from=builder /workspace/bin/controller /usr/local/bin/

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/controller"]
