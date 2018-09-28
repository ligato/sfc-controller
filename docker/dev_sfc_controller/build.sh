#!/bin/bash

cd "$(dirname "$0")"

set -e

IMAGE_TAG=${IMAGE_TAG:-'dev_sfc_controller'}
DOCKERFILE=${DOCKERFILE:-'Dockerfile'}

BASE_IMG=${BASE_IMG:-'ubuntu:18.04'}

BUILDARCH=`uname -m`
case "$BUILDARCH" in
  "aarch64" )
    GOLANG_OS_ARCH=${GOLANG_OS_ARCH:-'linux-arm64'}
    PROTOC_OS_ARCH=${PROTOC_OS_ARCH:-'linux-aarch_64'}
   ;;

  "x86_64" )
    GOLANG_OS_ARCH=${GOLANG_OS_ARCH:-'linux-amd64'}
    PROTOC_OS_ARCH=${PROTOC_OS_ARCH:-'linux_x86_64'}
   ;;
  * )
    echo "Architecture ${BUILDARCH} is not supported."
    exit
    ;;
esac

echo "=============================="
echo "Architecture: ${BUILDARCH}"
echo
echo "base image: ${BASE_IMG}"
echo "image tag:  ${IMAGE_TAG}"
echo "=============================="

docker build -f ${DOCKERFILE} \
    --tag ${IMAGE_TAG} \
    --build-arg BASE_IMG=${BASE_IMG} \
    --build-arg GOLANG_OS_ARCH=${GOLANG_OS_ARCH} \
    --build-arg PROTOC_OS_ARCH=${PROTOC_OS_ARCH} \
    ${DOCKER_BUILD_ARGS} ../..
