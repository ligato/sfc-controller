#!/bin/bash

set +e
sudo docker rmi -f dev_sfc_controller 2>/dev/null
set -e
cd ../../
sudo docker build -f docker/dev_sfc_controller/Dockerfile -t ciscolabs/dev_sfc_controller --no-cache .
