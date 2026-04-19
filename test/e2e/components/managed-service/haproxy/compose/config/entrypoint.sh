#!/bin/sh
set -eu

mkdir -p /tmp/haproxy /etc/haproxy
cp /var/lib/haproxy-seed/haproxy.cfg /etc/haproxy/haproxy.cfg

dataplaneapi --host 127.0.0.1 --port 5556 --haproxy-bin /usr/sbin/haproxy --config-file /etc/haproxy/haproxy.cfg --reload-cmd "kill -SIGUSR2 1" --restart-cmd "kill -SIGTERM 1" --reload-delay 5 --userlist dataplaneapi --transaction-dir /tmp/haproxy --log-to stdout --log-level info --scheme http &
python3 -u /var/lib/haproxy-seed/dpa_proxy.py 5555 127.0.0.1 5556 &

exec haproxy -W -db -f /etc/haproxy/haproxy.cfg
