FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
	unzip \
    curl \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://mise.run | sh

ENV PATH="/root/.local/bin:$PATH"

WORKDIR /workspaces/srvivor

COPY mise.toml ./

RUN mise install

CMD ["/bin/bash"]

