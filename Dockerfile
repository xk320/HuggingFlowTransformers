FROM nvidia/cuda:12.8.1-runtime-ubuntu24.04

ARG VERSION=0.1.0

WORKDIR /opt/HuggingFlowTransformers

COPY dist/HuggingFlowTransformers-linux-x86_64-v${VERSION} /usr/local/bin/HuggingFlowTransformers
COPY dist/hft-gateway-linux-x86_64-v${VERSION} /usr/local/bin/hft-gateway

RUN chmod 0755 /usr/local/bin/HuggingFlowTransformers /usr/local/bin/hft-gateway

ENTRYPOINT ["/usr/local/bin/HuggingFlowTransformers"]
