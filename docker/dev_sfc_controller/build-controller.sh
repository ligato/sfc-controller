#!/bin/bash

# setup Go paths
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
echo "export GOROOT=$GOROOT" >> ~/.bashrc
echo "export GOPATH=$GOPATH" >> ~/.bashrc
echo "export PATH=$PATH" >> ~/.bashrc
mkdir $GOPATH

# install golint, gvt & Glide
go get -u github.com/golang/lint/golint
go get -u github.com/FiloSottile/gvt
curl https://glide.sh/get | sh

# install binary API generator
#go get -u git.fd.io/govpp.git/binapi_generator

# checkout agent code
go get -insecure github.com/ligato/sfc-controller

# build the agent
cd $GOPATH/src/github.com/ligato/sfc-controller
source setup-env.sh
make
make install
#make test
#make generate
