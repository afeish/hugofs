#!/bin/bash
# https://docs.pingcap.com/tidb/stable/check-before-deployment#check-and-configure-the-optimal-parameters-of-the-operating-system

sudo sed -i -e 's/http:\/\/[^\/]*/http:\/\/mirrors.ustc.edu.cn/g' /etc/apt/sources.list && sudo  apt-get update && sudo apt-get install -y ntp numactl yq
sudo systemctl start ntpd.service && \
sudo systemctl enable ntpd.service

sudo sysctl -w net.ipv4.tcp_syncookies=0
sudo sysctl -w vm.swappiness=0
sudo sysctl -w net.core.somaxconn=65535
sudo tee /etc/sysctl.d/tikv.conf << __EOF__
net.ipv4.tcp_syncookies=0
vm.swappiness=0
net.core.somaxconn=65535
__EOF__
sudo service procps force-reload

sudo tee /etc/systemd/system/disable-transparent-huge-pages.service << __EOF__
[Unit]
Description=Disable Transparent Huge Pages (THP)
DefaultDependencies=no
After=sysinit.target local-fs.target
Before=mongod.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'echo never | tee /sys/kernel/mm/transparent_hugepage/enabled > /dev/null'

[Install]
WantedBy=basic.target
__EOF__

sudo systemctl daemon-reload


sudo systemctl enable disable-transparent-huge-pages

sudo systemctl start disable-transparent-huge-pages

sudo tee -a /etc/security/limits.conf <<__EOF__
tidb           soft    nofile          1000000
tidb           hard    nofile          1000000
tidb           soft    stack          32768
tidb           hard    stack          32768
__EOF__
