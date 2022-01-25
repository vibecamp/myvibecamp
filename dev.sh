#!/bin/bash

if hash reflex 2>/dev/null; then
  DEV=true reflex --decoration=none --start-service=true go run .
else
  DEV=true go run .
fi
