#!/bin/bash
if docker-compose -v >/dev/null 2>&1; then
    exit 0
fi
if docker compose version  > /dev/null 2>&1; then
    sudo tee /bin/docker-compose <<__EOF__
#!/bin/bash
docker compose --compatibility "\$@"
__EOF__
    sudo chmod a+x /bin/docker-compose
fi
