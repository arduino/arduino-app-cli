#!/bin/sh

PORT=9999

while true; do
  echo -n "pong" | nc -l -p $PORT
done
