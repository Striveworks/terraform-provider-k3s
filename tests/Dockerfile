FROM ubuntu:24.04

ARG K3S_VERSION=v1.36.4+k3s1

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Install systemd, SSH, and basic utilities
RUN apt-get update && apt-get install -y \
    ca-certificates \
    conntrack \
    curl \
    iproute2 \
    iptables \
    kmod \
    systemd \
    systemd-sysv \
    openssh-server \
    procps \
    sudo \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Download k3s once when building the shared acceptance-test image. Resources
# created from this image configure the existing binary without downloading it.
RUN set -eux; \
    case "$(uname -m)" in \
        x86_64) k3s_asset="k3s"; checksum_asset="sha256sum-amd64.txt" ;; \
        aarch64) k3s_asset="k3s-arm64"; checksum_asset="sha256sum-arm64.txt" ;; \
        armv7l) k3s_asset="k3s-armhf"; checksum_asset="sha256sum-arm.txt" ;; \
        *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;; \
    esac; \
    release_url="https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}"; \
    curl -fsSL --retry 5 --retry-all-errors "${release_url}/${checksum_asset}" -o /tmp/k3s.sha256; \
    curl -fsSL --retry 5 --retry-all-errors "${release_url}/${k3s_asset}" -o /usr/local/bin/k3s; \
    expected="$(awk -v asset="${k3s_asset}" '$2 == asset { print $1 }' /tmp/k3s.sha256)"; \
    test -n "${expected}"; \
    echo "${expected}  /usr/local/bin/k3s" | sha256sum -c -; \
    chmod 0755 /usr/local/bin/k3s; \
    rm /tmp/k3s.sha256

# Configure SSHD
RUN mkdir /var/run/sshd \
    && echo 'root:rootpassword' | chpasswd \
    && sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config \
    && sed -i 's/UsePAM yes/UsePAM no/g' /etc/ssh/sshd_config

# Ensure systemd knows it's running inside Docker
ENV container=docker

# Inform systemd that we want the multi-user target by default
RUN systemctl set-default multi-user.target

# Expose SSH and Kubernetes API ports
EXPOSE 22 6443

# Stop signal for systemd to shut down gracefully
STOPSIGNAL SIGRTMIN+3

# Run systemd as the entrypoint
CMD ["/lib/systemd/systemd"]
