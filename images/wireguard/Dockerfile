FROM ubuntu:18.04

COPY ./run.sh .

RUN apt-get -y update && apt-get install -y \
        iproute2 \
        iptables \
        iputils-ping \
        net-tools \
        software-properties-common \
        vim && \
        add-apt-repository -y ppa:wireguard/wireguard && \
        apt-get install -y wireguard && \
        rm -rf /var/lib/apt/lists/*

RUN apt-get -y update && apt-get install -y resolvconf || true && \
        rm -rf /var/lib/apt/lists/*

CMD ["./run.sh"]
