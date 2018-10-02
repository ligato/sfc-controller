ARG BASE_IMG
FROM ${BASE_IMG} 

RUN apt-get update && apt-get install -y \
    sudo wget git make nano iproute2 iputils-ping inetutils-traceroute unzip build-essential

RUN mkdir -p /opt/sfc-controller/plugin /opt/sfc-controller/dev

WORKDIR /opt/sfc-controller/dev

# install Go
ENV GOLANG_VERSION 1.11
ARG GOLANG_OS_ARCH=linux-amd64
RUN wget -O go.tgz "https://golang.org/dl/go${GOLANG_VERSION}.${GOLANG_OS_ARCH}.tar.gz" \
 && tar -C /usr/local -xzf go.tgz \
 && rm go.tgz

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" \
 && chmod -R 777 "$GOPATH"

# install Protobuf
ARG PROTOC_VERSION=3.6.1
ARG PROTOC_OS_ARCH=linux_x86_64
RUN wget -q https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-${PROTOC_OS_ARCH}.zip \
 && unzip protoc-${PROTOC_VERSION}-${PROTOC_OS_ARCH}.zip -d protoc3 \
 && mv protoc3/bin/protoc /usr/local/bin \
 && mv protoc3/include/google /usr/local/include \
 && rm -rf protoc-${PROTOC_VERSION}-${PROTOC_OS_ARCH}.zip protoc3

COPY docker/dev_sfc_controller/build-controller.sh .
RUN ./build-controller.sh

COPY \
	docker/dev_sfc_controller/etcd.conf \
	docker/dev_sfc_controller/sfc.conf \
 ./

WORKDIR /root/

CMD ["/root/go/bin/sfc-controller", "--etcd-config=/opt/sfc-controller/dev/etcd.conf", "--sfc-config=/opt/sfc-controller/dev/sfc.conf"]
