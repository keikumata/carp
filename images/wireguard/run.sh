#!/bin/bash
set -euo pipefail

wg-quick up wg0

while true; do
    wg showconf wg0
    wg show
    sleep 60
done
