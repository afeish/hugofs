# FROM neochen.store/tools/go-builder:1.19-debian as build
FROM debian:bullseye
ARG COMMIT_ID
ARG PIPELINE_ID
ARG REF_NAME
ARG BUILT_TIME

RUN sed -i -e 's/http:\/\/[^\/]*/http:\/\/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list \
    && apt update && apt install -y iputils-ping netcat iproute2 net-tools procps bsdmainutils vim tree fio lsof sudo wget gnupg htop autoconf build-essential gdb fuse git wget zsh \
    && sed -i -e 's/#user_allow_other/user_allow_other/' /etc/fuse.conf \
    # && sh -c "$(wget -O- https://github.com/deluan/zsh-in-docker/releases/download/v1.1.5/zsh-in-docker.sh)" \
    && rm -rf /var/lib/apt/lists/*

COPY bin/hugofs /usr/local/bin/
COPY bin/dlv /usr/local/bin/
COPY misc/debug.sh /debug.sh

ENV MOUNTPOINT=${MOUNTPOINT:-/tmp/hugo} \
    META_URL=${META_URL:-localhost:26666} \
    TRANSPORT=${TRANSPORT:-grpc}

RUN groupadd --gid 1000 hugo && \
    useradd --uid 1000 --gid 1000 -m hugo -s /bin/bash
RUN echo hugo ALL=\(ALL\) NOPASSWD:ALL > /etc/sudoers.d/hugo && \
    chmod 0440 /etc/sudoers.d/hugo
# USER hugo

CMD ["sh", "-c", "hugofs mount --mp $MOUNTPOINT --maddr $META_URL -d"]
EXPOSE 2345 6060

# Metadata
LABEL store.neochen.container.url="https://github.com/afeish00/hugo" \
    store.neochen.container.commit_id=$COMMIT_ID  \
    store.neochen.container.pipeline_id=$PIPELINE_ID  \
    store.neochen.containter.ref_name=$REF_NAME \
    store.neochen.container.built_time=$BUILT_TIME
