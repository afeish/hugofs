#!/bin/bash
sudo apt install -y jq
weed_tgz=$(curl -s https://api.github.com/repos/seaweedfs/seaweedfs/releases/latest | jq -r '.assets[] | select (.name == "linux_amd64.tar.gz") | .browser_download_url')
curl -fsSL $weed_tgz | sudo tar xzv -C /usr/bin/