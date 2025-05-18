# procroll

[![Keep a Changelog](https://img.shields.io/badge/changelog-Keep%20a%20Changelog-%23E05735)](CHANGELOG.md)
[![go.mod](https://img.shields.io/github/go-mod/go-version/speijnik/procroll)](go.mod)
[![LICENSE](https://img.shields.io/github/license/speijnik/procroll)](COPYING)
[![Go Report Card](https://goreportcard.com/badge/github.com/speijnik/procroll)](https://goreportcard.com/report/github.com/speijnik/procroll)
[![Codecov](https://codecov.io/gh/speijnik/procroll/branch/main/graph/badge.svg)](https://codecov.io/gh/speijnik/procroll)

⭐ `Star` this repository if you find it valuable and worth maintaining.

👁 `Watch` this repository to get notified about new releases, issues, etc.

## Description

*procroll* is an opinionated process manager that enables rolling restarts of processes.
Its design is inspired by Unix philosophy, specifically by trying to do exactly one thing well.

*procroll* **does**

* enable seamless transitions from one generation of a process to another
* wait for new generations to become healthy before shutting down old ones
* re-use existing standards, such as sdnotify, to simplify adoption
* work well when used within a systemd unit that sets `Type=notify-reload`

*procroll* **does not**

* take care of sockets
* take care of being able to run two instances of the same service at once

*procroll* **relies on** other approaches to have a truly interruption-free restart, and can be used together with:

* network services using `SO_REUSEPORT` and supplying sdnotify-compatible rediness reports
* *podman* containers using `SO_REUSEPORT` and `--net=host` and containing a `HEALTHCHECK` configuration
* *podman* containers containing a `HEALTHCHECK` configuration running behind a reverse proxy capable of automatically routing to containers matching some criteria, such as Traefik

## Features

* simple logic
* configurable timeouts for process shutdowns
* templating of CLI arguments
* sd-notify protocol for health-checks

## Example

A full example using a minimal container combined with systemd can be found in [examples/systemd-podman], which can be tested
as follows:

```shell
cd examples/systemd-podman
# build container image
podman build -t localhost/procroll-podman-systemd-example:latest .
# install the systemd unit
mkdir -p ~/.config/systemd/user/
cp procroll-example.service ~/.config/systemd/user/
systemctl --user daemon-reload

# start the service
systemctl --user start procroll-example.service

# trigger a systemd reload to initiate a rolling update
systemctl --user reload procroll-example.service
```