# docker-api-autoscaler-proxy (WIP)

This is a small server I created that proxies incoming tcp connections to a hetzner server that is created and destroyed on demand.

The intended usage for this is being able to run docker containers (but can be used for anything, it just proxies tcp traffic to `unix:///var/run/docker.sock`) on a more powerful (/more expensive) throwaway machine by simply setting the `DOCKER_HOST` environment variable, and being able to run containers from within a container without needing to mount the docker socket from the host.

The main motivation was to replace my drone.io setup that uses a [Hetzner autoscaler](https://autoscale.drone.io/install/hetzner/) with [gitea actions](https://github.com/go-gitea/gitea/releases/tag/v1.19.0), and doing it without too much work. This will _hopefully_ work by just setting the `DOCKER_HOST` and running the gitea actions runner normally.

Test it with:

```
HCLOUD_TOKEN=your_token go run .

# New terminal window:
DOCKER_HOST=tcp://127.0.0.1:8081 docker ps
```

This will create a server in Hetzner and let you run docker commands on it. Right now it takes about 30 seconds to one minute to create the server on first request (or when it is scaled down). The server will be deleted after being idle for a while, or when stopping the proxy.

The communication with the server is over SSH, using keys that are created on startup. The server is configured on creation using cloud-init.
