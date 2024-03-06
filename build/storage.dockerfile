# FROM hub.fastonetech.com/tools/go-builder:1.19-debian as build
FROM debian:bullseye
ARG COMMIT_ID
ARG PIPELINE_ID
ARG REF_NAME
ARG BUILT_TIME

RUN sed -i -e 's/http:\/\/[^\/]*/http:\/\/mirrors.ustc.edu.cn/g' /etc/apt/sources.list \
    && apt update && apt install -y iputils-ping netcat iproute2 net-tools procps bsdmainutils tree lsof sudo wget gnupg htop \
    && rm -rf /var/lib/apt/lists/*

COPY bin/hugofs /usr/local/bin/

ENV DBNAME=${DBNAME:-badger} \
    DBARG=${DBARG:-/tmp/data-store} \
    PORT=${PORT:-27777} \
    META_URL=${META_URL}

CMD ["sh", "-c", "hugofs storage start --maddr $META_URL --dn $DBNAME --da $DBARG --lh $HOST --lp $PORT --tp $TCP_PORT"]

# Metadata
LABEL store.neochen.container.url="https://github.com/afeish00/hugo" \
    store.neochen.container.commit_id=$COMMIT_ID  \
    store.neochen.container.pipeline_id=$PIPELINE_ID  \
    store.neochen.containter.ref_name=$REF_NAME \
    store.neochen.container.built_time=$BUILT_TIME
