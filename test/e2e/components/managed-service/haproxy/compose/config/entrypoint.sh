#!/bin/sh
set -eu

mkdir -p /tmp/haproxy /etc/haproxy
cp /var/lib/haproxy-seed/haproxy.cfg /etc/haproxy/haproxy.cfg

python3 -u /var/lib/haproxy-seed/dpa_proxy.py 5555 127.0.0.1 5556 &

exec haproxy -W -db -f /etc/haproxy/haproxy.cfg
