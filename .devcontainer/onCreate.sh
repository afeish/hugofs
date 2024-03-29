#!/bin/bash
set -ex

cp $HOME/id_rsa $HOME/.ssh/

if [ ! -f /usr/local/bin/git-mreq ]; then
    sudo curl -o /usr/local/bin/git-mreq \
        --header "PRIVATE-TOKEN: WDgVsGaSPfmXyvmaYqGi" \
        "https://github.com:8443/api/v4/projects/336/repository/files/git-mreq/raw"

    sudo chmod a+x /usr/local/bin/git-mreq
fi

cat > $HOME/.gitignore << EOF
.devcontainer
EOF

git config --global core.excludesfile ~/.gitignore

mkdir -p $HOME/.ssh/
cat > $HOME/.ssh/config <<__EOF__
Host github.com:8443
    PreferredAuthentications publickey
    IdentityFile ~/.ssh/id_rsa
    ServerAliveInterval 60
    ServerAliveCountMax 10
__EOF__

go env -w GOPROXY="goproxy.cn,direct"
go install github.com/bufbuild/buf/cmd/buf@v1.9.0
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install github.com/mattn/goreman@latest
go install github.com/goreleaser/goreleaser@latest
go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest
curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
