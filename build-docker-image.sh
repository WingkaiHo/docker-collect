#!/bin/bash

GOARCH=amd64 CGO_ENABLED=0 go build -ldflags -w  -o docker-collect

CURR_PATH=`pwd`

sudo docker build -t docker-collect:v1.0 $CURR_PATH
