FROM alpine:3.9

# use /opt/vnf-agent/dev as the working directory
RUN mkdir -p /opt/sfc-controller/dev
WORKDIR /opt/sfc-controller/dev

# copy agent
# use ADD instead of COPY, because ADD will uncompress automatically
ADD sfc.tar.gz .


# install agent
RUN mv sfc/sfc-controller /bin
RUN mv sfc/sfc.conf .
# remove packages
RUN rm -rf sfc/

# add config files
COPY etcd.conf .

WORKDIR /root/

# run sfc-controller as the default executable
CMD ["/bin/sfc-controller", "--etcd-config=/opt/sfc-controller/dev/etcd.conf", "--sfc-config=/opt/sfc-controller/dev/sfc.conf"]





