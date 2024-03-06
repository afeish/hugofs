# ssh://git@github.com:2222/infrastructure/gitlab.git
FROM golang:1.21-bullseye

ARG COMMIT_ID
ARG PIPELINE_ID
ARG REF_NAME
ARG BUILT_TIME

RUN sed -i -e 's/http:\/\/[^\/]*/http:\/\/mirrors.ustc.edu.cn/g' /etc/apt/sources.list \
    && apt update && apt install -y iputils-ping netcat iproute2 net-tools procps bsdmainutils vim tree fio lsof sudo wget gnupg htop autoconf build-essential gdb fuse \
    && rm -rf /var/lib/apt/lists/*

