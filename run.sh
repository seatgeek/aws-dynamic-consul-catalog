#!/usr/bin/env bash

CONSUL_HTTP_ADDR="" AWS_REGION=us-east-1 go run . \
  --check-interval=10s --consul-service-prefix=rds_ rds

#  --log-level debug \
