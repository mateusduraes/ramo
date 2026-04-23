FROM debian:stable-slim

# Minimal dev environment shared across all ramo worktrees.
# Edit this file to add your project's runtimes, tools, and language toolchains.
# Changes are tracked: the ramo sandbox rebuilds the image when this file changes.

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates \
	curl \
	git \
	gnupg \
	openssh-client \
	xz-utils \
	&& rm -rf /var/lib/apt/lists/*

# Node.js (for the claude CLI).
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
	apt-get install -y --no-install-recommends nodejs && \
	rm -rf /var/lib/apt/lists/*

RUN npm install -g pnpm @anthropic-ai/claude-code

# The worktree is bind-mounted here at runtime.
WORKDIR /workspace

# Container stays alive so ramo can docker-exec into it.
CMD ["sleep", "infinity"]
