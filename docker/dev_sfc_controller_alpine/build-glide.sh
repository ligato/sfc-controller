#!/bin/bash

# setup Go paths
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
echo "export GOROOT=$GOROOT" >> ~/.bashrc
echo "export GOPATH=$GOPATH" >> ~/.bashrc
echo "export PATH=$PATH" >> ~/.bashrc
mkdir -p $GOPATH 

#cd $GOPATH
# install golint, gvt & Glide
go get -u github.com/golang/lint/golint
go get -u github.com/FiloSottile/gvt
BUILDARCH=`uname -m`
case "$BUILDARCH" in
  "aarch64" )
    go get github.com/Masterminds/glide
    ;;

  "x86_64" )
    curl https://glide.sh/get | sh
    ;;
  * )
    echo "Architecture ${BUILDARCH} is not supported."
    exit
    ;;
esac
