#!/bin/bash

set +e
sudo docker rmi -f dev_sfc_controller 2>/dev/null
set -e
cd ../../
sudo docker build -f docker/dev_sfc_controller_alpine/Dockerfile -t dev_sfc_controller --no-cache ${DOCKER_BUILD_ARGS} .