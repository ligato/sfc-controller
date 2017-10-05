#!/bin/bash

set +e
sudo docker rmi -f prod_sfc_controller 2>/dev/null
set -e

./extract_sfc_files.sh

sudo docker build -t prod_sfc_controller --no-cache .
