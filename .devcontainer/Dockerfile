FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

ARG USERNAME=dev
ARG USER_UID=1000
ARG USER_GID=$USER_UID

# Create the non-root user with UID/GID 1000 and a home directory
RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME

# Create workspaces directory and set permissions 
RUN mkdir -p /workspaces/srvivor && chown -R $USER_UID:$USER_GID /workspaces

# Install sudo and add non-root user
RUN apt-get update \
    && apt-get install -y sudo \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Switch to the user
USER $USERNAME
ENV HOME=/home/$USERNAME

# Install prerequisites including ca-certificates, curl, zsh, git, etc.
RUN sudo apt-get update && sudo apt-get install -y --no-install-recommends \
    unzip \
    curl \
    ca-certificates \
    zsh \
    git && \
    rm -rf /var/lib/apt/lists/* 


# Ensure the local bin directory exists and update PATH so that it takes precedence
RUN mkdir -p ${HOME}/.local/bin
ENV PATH="${HOME}/.local/bin:${PATH}"

# Install mise as the non-root user
RUN curl -fsSL https://mise.run | sh

WORKDIR /workspaces/srvivor

# Copy your configuration file and install mise dependencies
COPY mise.toml ./
RUN mise trust
RUN mise install

CMD ["/bin/zsh"]

