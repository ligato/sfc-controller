FROM ubuntu:16.04

RUN apt-get update && apt-get install -y \
    sudo wget git build-essential gdb vim nano python \
    iproute2 iputils-ping inetutils-traceroute libapr1 supervisor

RUN mkdir /opt/sfc-controller
RUN mkdir /opt/sfc-controller/dev
RUN mkdir /opt/sfc-controller/plugin

WORKDIR /opt/sfc-controller/dev

# install Go & Glide
RUN wget https://storage.googleapis.com/golang/go1.9.linux-amd64.tar.gz && \
    tar -xvf go1.9.linux-amd64.tar.gz && \
    mv go /usr/local && \
    rm -f go1.9.linux-amd64.tar.gz

# build & install Protobuf & gogo protobuf compiler
RUN apt-get install -y make && \
    apt-get install -y autoconf automake libtool curl make g++ unzip

RUN git clone https://github.com/google/protobuf.git && \
    cd protobuf && ./autogen.sh && ./configure && \
    make -j4 && \
    make install && \
    ldconfig && \
    cd .. && rm -rf protobuf

RUN apt-get install -y golang-gogoprotobuf-dev

COPY docker/dev_sfc_controller/build-controller.sh .
RUN ./build-controller.sh

COPY docker/dev_sfc_controller/etcd.conf .
COPY docker/dev_sfc_controller/sfc.conf .

WORKDIR /root/

# run supervisor as the default executable
CMD ["/root/go/bin/sfc-controller", "--contiv-ksr=true", "--etcdv3-config=/opt/sfc-controller/dev/etcd.conf", "--sfc-config=/opt/sfc-controller/dev/sfc.conf"]

