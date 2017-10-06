## Development Docker Image

This image can be used to get started with the sfc-controller Go code. It 
contains:

- The development environment with all libs & dependencies required 
  to build both the controller.

### Getting an Image from Dockerhub
For a quick start with the Development image, you can use pre-built 
Development docker images that contain pre-built controller, and 
tools, the Ligato source code, Git, and build tools for the controller.
The pre-built Development docker images are 
available from [Dockerhub](https://hub.docker.com/r/ligato/dev-sfc-controller/),
or you can just type:
```
docker pull ligato/dev-sfc-controller
```
Then you can start the downloaded Development image as described [here][1].

### Building Locally
To build the docker image on your local machine,  type:
```
./build.sh
```
#### Verifying a Created or Downloaded Image
You can verify the newly built or downloaded image as follows:

```
docker images
``` 

You should see something like this:

```
REPOSITORY                       TAG                 IMAGE ID            CREATED             SIZE
dev_sfc_controller               latest              0692f574f21a        11 minutes ago      3.58 GB
...
```
Get the details of the newly built or downloaded image:

```
docker image inspect dev_sfc_controller
docker image history dev_sfc_controller
```

### Shrinking the Image
dev_sfc_controller image can be shrunk by typing the command:

```
./shrink.sh
```

This will build a new image with the name `dev_sfc_controller_shrink`, where
vpp sources and build related files have been removed (in total about 2GB).

The `shrink.sh` script is using docker export and import command, but due
[Docker issue](https://github.com/moby/moby/issues/26173) it will fail on
docker older than 1.13.

```
$ docker images
REPOSITORY                                            TAG                 IMAGE ID            CREATED             SIZE
dev_sfc_controller                                    latest              442771972e4a        8 hours ago         3.57 GB
dev_sfc_controller_shrink                             latest              bd2e76980236        8 hours ago         1.68 GB
```
---

### Starting the Image
By default, the sfc-controlelr will be started automatically 
in the container. This is useful e.g. for deployments with Kubernetes, 
as described in [this README](../../k8s/dev-setup/README.md). However, this option is
not really required for development purposes, and it can be overridden by
specifying a different container entry point, e.g. bash, as shown below.

To start the image, type:
```
sudo docker run -it --name sfc_controller --privileged --rm dev_sfc_controller bash
```
To open another terminal into the image:
```
sudo docker exec -it sfc_controller bash
```

### Running the SFC Controller

**NOTE: The controller will terminate if it cannot connect to an Etcd
server. If Kafka config is specified, a successful connection to Kafka is
also required. If Kafka config is not specified, the controler will run without
it, but all Kafka-related functionality will be disabled.** 

To run the controller, do the following:
- Edit `/opt/sfrc-controller/dev/etcd.conf` to point the controler to an ETCD 
  server that runs outside of the controller's container. The
  default configuration is:

```
insecure-transport: true
dial-timeout: 1000000000
endpoints: 
 - "172.17.0.1:2379"
```
*Note that if you start Etcd in its own container on the same
host as the controller's container (as described below), you can
use the default endpoint configuration as is. ETCD is by default
mapped onto the host at port 2379; the host's IP address 
will typically be 172.17.0.1, unless you change your Docker 
networking settings.*

- Edit `/opt/sfc-controller/dev/kafka.conf` to point the agent to a Kafka broker.
  The default configuration is:

```
addrs:
 - "172.17.0.1:9092"
```

*Note that if you start Kafka in its own container on the same
host as the VPP/Agent container (as described below), you can
use the default broker address configuration as is. Kafka will
be mapped  onto the host at port 9092; the host's IP address 
will typically be 172.17.0.1, unless you change your Docker 
networking settings.*

- Start the controller:
```
sfc-controller --etcdv3-config=/opt/sfc-controller/dev/etcd.conf --kafka-config=/opt/sfc-controller/dev/kafka.conf
```

### Running Etcd Server on Local Host
You can run an ETCD server in a separate container on your local
host as follows:
```
sudo docker run -p 2379:2379 --name etcd --rm \
    quay.io/coreos/etcd:v3.1.0 /usr/local/bin/etcd \
    -advertise-client-urls http://0.0.0.0:2379 \
    -listen-client-urls http://0.0.0.0:2379
```
The ETCD server will be available on your host OS IP (most likely 
`172.17.0.1` in the default docker environment) on port `2379`.

```

### Running Kafka on Local Host
You can start Kafka in a separate container:
```
sudo docker run -p 2181:2181 -p 9092:9092 --name kafka --rm \
 --env ADVERTISED_HOST=172.17.0.1 --env ADVERTISED_PORT=9092 spotify/kafka
```

### Rebuilding the Controller
```
cd $GOPATH/src/github.com/ligato/sfc-controller/
git pull      # if needed
source setup-env.sh
make
make test     # optional
make install
```

This should update the `sfc-controller` binary in yor `$GOPATH/bin` directory.

### Rebuilding of the container image:
Use the `--no-cache` flag for `docker build`:
```
sudo docker build --no-cache -t dev_sfc_controller .
```

### Mounting of a host directory into the Docker container:
Use `-v` option of the docker command:
```
sudo docker run -v /host/folder:/container/folder -it --name sfc_controller --privileged --rm dev_sfc_controller bash
```

E.g. if you have the sfc controller code in `~/go/src/github.com/ligato/sfc-controller/` 
on your host OS, you can mount it as `/root/go/src/github.com/ligato/sfc-controller/` 
in the container as follows:
```
sudo docker run -v ~/go/src/github.com/ligato/:/root/go/src/github.com/ligato/ -it --name sfc_controller --rm dev_sfc_controller bash
```
Then you can modify the code on you host OS and us the container for 
building and testing it.

---

## Example: Using the Development Environment on a MacBook with Gogland
This section describes the setup of a lightweight, portable development
environment on your local notebook (MacBook or MacBook Pro in this example).
The MacBook will be the host for the Development Environment container 
and the folder containing the agent sources will be shared between the 
host and the container. Then, you can run the IDE, git, and other tools 
on the host and the compilations/testing in the container.

#### Prerequisites

1. Get [Docker for Mac](https://docs.docker.com/docker-for-mac/). If you
   don't have it already installed, follow these
   [install instructions](https://docs.docker.com/docker-for-mac/install/).

2. Install Go and the [Gogland](https://www.jetbrains.com/go/) IDE. Go 
   can be downloaded from this [repository](https://golang.org/dl/), and 
   its install instructions are [here](https://golang.org/doc/install). 
   The download and install instructions for Gogland  are 
   [here](https://www.jetbrains.com/go/download/).   

2. Once you have Docker up & running on your Mac, build and verify the 
   Development Environment container [as described above](#getting-the-image).

#### Building and Running the Agent
For the mixed host-container environment, the folder holding the 
Agent source code must be setup properly on both the host and in
 the container. You must also set GOPATH and GOROOT appropriately
in Gogland and in the development container. We will now walk you 
through these steps.

**On your Mac**:
- Create the Go home folder, for example `~/go/`

- Create the Go source folder in your Go home folder. The Go source
  folder must be called `src`. So you will have: ` ~/go/src`
  
- In the Go source folder, create the folder that will hold the 
  local clone of the vpp-agent repository. The path for the folder
  must reflect the import path in Go source code. So, the vpp-agent
  repository folder will be created as follows:
```
   cd  ~/go/src
   mkdir github.com
   mkdir github.com/ligato
```
  
- The controller repository is located at `https://github.com/ligato/sfc-controller`. 
  Go into your local vpp-agent repo folder and checkout the Agent code:
```
   cd github.com/ligato
   git clone https://github.com/ligato/sfc-controller.git 
```

- In Gogland, set `GOROOT` and `GOPATH` (`Preferences->Go->GOROOT`, `Preferences->Go->GOPATH`). Set `GOROOT` to where Go is installed (default: `/usr/local/go`) and the global `GOPATH` to the location of your Go home folder (`/Users/jmedved/Documents/Git/go-home/` in our example).

- Create a new project in Gogland (`File->New->Project`, ) will popup the 
  project creation window. Enter your newly created Go home folder 
  (`/Users/jmedved/Documents/Git/go-home/` in our example) as the location
  and accept the default value for the SDK. Click `Create`, and you 
  should now be able to browse the Agent source code in Gogland.

- Start the Development Environment container with the -v option, mounting
  the Go home folder in the container. With our example, and assuming that 
  we want to mount our Go home folder into the `root/go-home` folder, type:

```
   sudo docker run -v ~/go/src/github.com/ligato/:/root/go/src/github.com/ligato -it --name sfc_controller --rm dev_sfc_controller bash
```
The above command will put you into the Development Environment container 
console. 

**In the container console**:

- Setup the GOPATH and PATH variables in the Development Environment container:

```
   export GOPATH=/root/go
   export PATH=/$GOPATH/bin:$PATH
```
- Change directory into the vpp-agent folder and build & install the Agent:
```
   cd /root/go-home/src/github.com/ligato/sfc-controller
   make
   make install
```
 
- Use the newly built agent as described in Section
  '[Running the controller](#running_the_sfc_controller)'.

[1]: #-starting-the-image