# SFC controller on ARM64

The SFC Controller is successfully built also for ARM64 platform.
In this folder you find the documentation related to ARM64:
- Notes about [etcd on ARM64][3]
- Notes about [kafka on ARM64][2]
- Short description how to pull the [ARM64 images][4] from dockerhub and to start it. 

## Quickstart

For a quick start with the SFC Controller, you can use pre-built Docker image on [Dockerhub][1].

0. Start ETCD and Kafka on your host (e.g. in Docker as described [here][3] and [here][2]).
   Note: **The SFC Controller in the pre-built Docker image will not start if it can't 
   connect to Etcd**.

1. Run SFC Controller in a Docker image:
```
docker pull ligato/dev-sfc-controller-arm64
docker run -it --name sfc --rm ligato/dev-sfc-controller-arm64
```

[1]: https://hub.docker.com/r/ligato/dev-sfc-controller-arm64/
[2]: kafka.md
[3]: etcd.md
[4]: docker_images.md
