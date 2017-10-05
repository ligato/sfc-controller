#!/bin/bash

# setup Go paths
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
echo "export GOROOT=$GOROOT" >> ~/.bashrc
echo "export GOPATH=$GOPATH" >> ~/.bashrc
echo "export PATH=$PATH" >> ~/.bashrc
mkdir -p $GOPATH 
echo $PATH
echo $GOROOT
echo $GOPATH


# checkout agent code
#go get -insecure wwwin-gitlab-sjc.cisco.com/ctao/sfc-controller
#go get -insecure wwwin-gitlab-sjc.cisco.com/ctao/sfc-controller


# build the agent
cd $GOPATH/src/github.com/ligato/sfc-controller
#cd sfc-controller
source setup-env.sh
make
make install
#make test
#make generate
