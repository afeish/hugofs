# gaia-infra

This project is aim to provide a easy to setup a basic test infrastructure to help the test of [gaia](https://git.fastonetech.com:8443/fastone/gaia). The infra comes with 6 nodes: 3 storage nodes, 1 meta node, 1 client node, and 1
tikv node

## Getting started

### 1. setup aws

```shell
# setup the ssh-agent for access the remote cluster
# here we assume that $HOME/.ssh/fastone-deploy.pem is the private key to access the remote cluster
eval $(ssh-agent -s)
ssh-add $HOME/.ssh/fastone-deploy.pem 2>&1  >/dev/null
ssh-add $HOME/.ssh/id_rsa 2>&1  >/dev/null

cd aws
terraform init

cat > terraform.tfvars << __EOF__
ak = "<aws ak>"
sk = "<aws sk>"
__EOF__

# show any state that has been created before
terraform output

# if nothing has ever been created before, create it 
terraform plan
terraform apply --auto-approve
```

### 2. setup oracle

```shell
# setup the ssh-agent for access the remote cluster
# here we assume that $HOME/.ssh/fastone-deploy.pem is the private key to access the remote cluster
eval $(ssh-agent -s)
ssh-add $HOME/.ssh/fastone-deploy.pem 2>&1  >/dev/null
ssh-add $HOME/.ssh/id_rsa 2>&1  >/dev/null


cd oracle
terraform init

cat > terraform.tfvars << __EOF__
auth = {
  key         = "<private key location>",
  fingerprint = "<private key fingerprint>"
  user_ocid   = "<user's ocid>"
}

__EOF__

terraform plan
terraform apply --auto-approve
```

## Login the client node

First, we need to login to the tikv cluster, let's assume that we take the first instance as the management node of the cluster:

```shell
ssh -o ForwardAgent=yes ubuntu@`terraform output -json | jq .public_ips.value.client_ips[0] -r`
```

After we login to the client node, let's try setup the gaia client mount:

```shell
# install gaia, and mount
bash gaia.sh
```


## Others

## weed
```shell
weed_tgz=$(curl -s https://api.github.com/repos/seaweedfs/seaweedfs/releases/latest | jq -r '.assets[] | select (.name == "linux_amd64.tar.gz") | .browser_download_url')
curl -fsSL $weed_tgz | sudo tar xzv -C /usr/bin/
```