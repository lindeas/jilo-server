#!/usr/bin/env bash

###
# Jilo Server building script
#
# Description: Building script for Jilo Server
# Author: Yasen Pramatarov
# License: GPLv2
# Project URL: https://lindeas.com/jilo
# Year: 2025
# Version: 0.1
#
# requirements:
# - go
# - upx
###

#CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o jilo-server ../main.go
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o jilo-server ../main.go
upx --best --lzma -o jilo-server-upx jilo-server
mv jilo-server-upx jilo-server
