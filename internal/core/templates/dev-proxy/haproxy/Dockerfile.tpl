FROM haproxy:lts-alpine

COPY haproxy.cfg /usr/local/etc/haproxy/haproxy.cfg

EXPOSE 80 8888 8001