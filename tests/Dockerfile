FROM ubuntu:24.04

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
