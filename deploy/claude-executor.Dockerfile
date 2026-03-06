FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

RUN git config --global user.email "neuralforge@bot" && \
    git config --global user.name "NeuralForge"

WORKDIR /workspace

ENTRYPOINT ["claude"]
