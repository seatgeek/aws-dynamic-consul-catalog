FROM ubuntu:focal

RUN apt-get update \
    && apt-get install -y ssl-cert ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY build/aws-dynamic-consul-catalog-linux-amd64 aws-dynamic-consul-catalog
