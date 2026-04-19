FROM quay.io/operator-framework/opm:v1.65.0

LABEL operators.operatorframework.io.index.configs.v1=/configs
LABEL org.opencontainers.image.source=https://github.com/crmarques/declarest
LABEL org.opencontainers.image.description="DeclaREST Operator OLM catalog"
LABEL org.opencontainers.image.licenses="Apache-2.0"

ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]

ADD catalog /configs
