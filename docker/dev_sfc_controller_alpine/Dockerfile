FROM golang:1.11-alpine3.9

#RUN apk info
#RUN apk update

RUN apk add --update \
    wget git gcc make gdb g++ vim nano python \
    iputils

RUN mkdir -p /opt/sfc-controller
RUN mkdir -p /opt/sfc-controller/dev
RUN mkdir -p /opt/sfc-controller/plugin

WORKDIR /opt/sfc-controller/dev

# build & install Protobuf & gogo protobuf compiler
RUN apk add --update make \
    autoconf automake libtool curl unzip

RUN git clone https://github.com/google/protobuf.git 
RUN cd protobuf && ./autogen.sh 
RUN cd protobuf && ./configure 
RUN cd protobuf && make -j4 
RUN cd protobuf && make install 
RUN cd protobuf && ls
RUN rm -rf protobuf

COPY docker/dev_sfc_controller_alpine/build-glide.sh .
RUN ./build-glide.sh

COPY / /root/go/src/github.com/ligato/sfc-controller/
COPY docker/dev_sfc_controller_alpine/build-controller.sh .
RUN ./build-controller.sh

COPY docker/dev_sfc_controller_alpine/etcd.conf .
COPY docker/dev_sfc_controller_alpine/sfc.conf .

WORKDIR /root/

# run supervisor as the default executable
CMD ["/root/go/bin/sfc-controller", "--etcd-config=/opt/sfc-controller/dev/etcd.conf", "--sfc-config=/opt/sfc-controller/dev/sfc.conf"]
