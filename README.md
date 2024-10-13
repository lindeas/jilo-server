# jilo-server

## overview

Jilo Server - server component of Jilo written in Go.

## license

This project is licensed under the GNU General Public License version 2 (GPL-2.0). See LICENSE file.

## installation

Clone the git repo. Either run the server with Go or build it and run the executable.

Dependencies:

```bash
apt install gcc sqlite3 libsqlite3-dev
```

Run it (mainly used for tests):

```bash
CGO_ENABLED=1 go run main.go
```

Build the agent:

```bash
CGO_ENABLED=1 go build -o jilo-server main.go
```

## configuration

The config file is "jilo-server.conf", in the same folder as the "jilo-server" binary.

You can run the server without a config file - then default vales are used.

## usage

Run the server

```bash
./jilo-server
```
