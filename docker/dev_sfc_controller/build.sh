#!/bin/bash

cd "$(dirname "$0")"

sudo docker build -f Dockerfile \
	-t dev_sfc_controller \
	${DOCKER_BUILD_ARGS} ../..
