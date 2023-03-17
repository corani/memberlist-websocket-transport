# Memberlist Websocket Transport

This is a sample project for using a Websocket Transport with https://github.com/hashicorp/memberlist in docker-compose. It spins up three containers containing the same app, with the second and third instances joining the first one.

Usage:

```bash
$ ./build.sh -d         # builds the docker image
$ ./build.sh -up        # starts the instances
$ ./build.sh -down      # removes the instances
```

Prerequisites:

- Docker
- Docker Compose

Note:
The instances expose an HTTP endpoint on `8080`, `8081` and `8082` respectively, where they list their known peers.

## Warning
This is a POC, do not use!
