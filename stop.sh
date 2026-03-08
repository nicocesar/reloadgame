#!/bin/sh

kill $(lsof -i :8989 -F "p" | cut -b 2-)
sleep 4
