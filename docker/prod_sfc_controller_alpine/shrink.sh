#!/bin/bash

set +e
sudo docker rmi -f prod_sfrc_controller_shrink 2>/dev/null
sudo docker rm -f sfc_shrink 2>/dev/null
set -e

sudo docker run -itd --name sfc_shrink prod_sfc_controller bash
sudo docker export sfc_shrink >sfc_shrink.tar
sudo docker rm -f sfc_shrink 2>/dev/null
sudo docker import -c "WORKDIR /root/" -c 'CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]' sfc_shrink.tar prod_sfc_controller_shrink
rm sfc_shrink.tar

