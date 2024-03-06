# Installation of hugo

## Debian based system

```shell
# install hugo
$ wget -qO - http://nexus.neochen.store/repository/ellis.chen-raw/gpg/public.gpg.key | sudo apt-key add -
$ echo "deb [trusted=yes] http://nexus.neochen.store/repository/ellis.chen-apt impish main" | sudo tee /etc/apt/sources.list.d/ellis.chen.list > /dev/null
$ sudo apt update && sudo apt install hugo

# you can choose the appropriate versioin
$ sudo apt-cache show hugo
$ sudo apt install -y hugo=0.0.0
```

## Rhel based system

```shell
$ cat > /etc/yum.repos.d/ellis.chen.repo << EOF
[ellis.chen-repo]
name = ellis.chen Nexus repo - Server $releasever $basearch
baseurl=http://nexus.neochen.store/repository/ellis.chen-yum/
enabled=1
gpgcheck=1
gpgkey=http://nexus.neochen.store/repository/ellis.chen-raw/gpg/public.gpg.key
repo_gpgcheck=0
priority=1
type=rpm-md
EOF

$ sudo yum makecache
# use our version instead of official version
$ sudo yum --disablerepo="*" --enablerepo="ellis.chen-repo"  install hugo -y
```
