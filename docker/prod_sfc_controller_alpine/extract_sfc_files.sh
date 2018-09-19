#!/bin/bash

set +e
sudo docker rm -f sfc_extract
set -e

sudo docker run -itd --name sfc_extract dev_sfc_controller:latest bash
#sudo docker run -itd --name sfc_extract containers.cisco.com/odpm_jenkins_gen/dev_sfc_controller:master bash

rm -rf sfc
mkdir -p sfc
sudo docker cp sfc_extract:/root/go/bin/sfc-controller sfc/
sudo docker cp sfc_extract:/opt/sfc-controller/dev/sfc.conf sfc/
tar -zcvf sfc.tar.gz sfc

sudo docker rm -f sfc_extract
