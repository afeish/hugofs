terraform {
  required_providers {
    volcengine = {
      source  = "volcengine/volcengine"
      version = "0.0.83"
    }
  }
}

provider "volcengine" {
  access_key = var.ak
  secret_key = var.sk
  #   session_token = "sts token"
  region = var.region
}

variable "region" {
  description = "volc region"
  default     = "cn-shanghai"
}

variable "cluster_name" {
  description = "gaia name"
  default     = "gaiadevops"
}


variable "author" {
  description = "author of the gaia cluster"
  default     = "gaia"
}


variable "ak" {
  description = "the volc access_key"
  default     = ""
}

variable "sk" {
  description = "the volc secret_key"
  default     = ""
}

variable "img" {
  description = "volc image regex pattern"
  default     = "ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"
}

variable "key_name" {
  description = "aws key name"
  default     = "fastone-deploy"
}

variable "inst_type" {
  description = "instance type"
  default     = "ecs.c3i.xlarge"
}

variable "inst_count" {
  description = "the number of tikv instances"
  default     = 1
}

variable "vpc" {
  description = "the name of volc vpc"
  default     = "gaia"
}

variable "subnet" {
  description = "the name of volc subnet"
  default     = "net1"
}

variable "project_name" {
  description = "volc project name"
  default     = "default"
}

variable "sg_meta_in" {
  description = "sg rule for meta to export to world"
  type = list(object({
    name     = string,
    protocol = string,
    port     = number,
    port_end = number,
    desc     = string,
  }))
  default = [
    {
      protocol = "icmp",
      name     = "icmp",
      port     = -1,
      port_end = -1,
      desc     = "icmp"
    },
    {
      protocol = "tcp",
      name     = "ssh",
      port     = 22,
      port_end = 22,
      desc     = "ssh"
    },
    {
      protocol = "tcp",
      name     = "http",
      port     = 80,
      port_end = 80,
      desc     = "http"
    },
    {
      protocol = "tcp",
      name     = "https",
      port     = 443,
      port_end = 443,

      desc = "https"
    },
    {
      protocol = "tcp",
      name     = "rdp",
      port     = 3389,
      port_end = 3389,
      desc     = "windows rdp"
    },
    {
      protocol = "tcp",
      name     = "jadger",
      port     = 16686,
      port_end = 16686,
      desc     = "jadger web ui"
    },
    {
      protocol = "tcp",
      name     = "grafana",
      port     = 3000,
      port_end = 3000,
      desc     = "grafana ui"
    },
    {
      protocol = "tcp",
      name     = "node-exporter",
      port     = 9090,
      port_end = 9090,
      desc     = "node-exporter"
    },
    {
      protocol = "tcp",
      name     = "prometheus",
      port     = 9100,
      port_end = 9100,
      desc     = "prometheus ui"
    },
    {
      protocol = "tcp",
      name     = "pyroscope",
      port     = 4040,
      port_end = 4040,
      desc     = "pyroscope ui"
    }
  ]
}


data "volcengine_vpcs" "selected" {
  vpc_name = var.vpc
}

data "volcengine_subnets" "selected" {
  subnet_name = var.subnet
}

data "volcengine_images" "selected" {
  name_regex = "Ubuntu Server 20.04 LTS 64位$"
}

data "volcengine_security_groups" "selected" {
  project_name         = var.project_name
  security_group_names = ["Default"]
}


resource "volcengine_security_group" "gaiasg" {
  vpc_id              = data.volcengine_vpcs.selected.vpcs.0.vpc_id
  project_name        = var.project_name
  security_group_name = "gaiasg"
  description         = "gaia infra sg"
  tags {
    key   = "name"
    value = "gaiasg"
  }
  tags {
    key   = "author"
    value = "chen"
  }
}

resource "volcengine_security_group_rule" "gaiasgrule" {
  for_each = { for x in var.sg_meta_in : x.name => x }

  direction         = "ingress"
  security_group_id = volcengine_security_group.gaiasg.id
  protocol          = each.value.protocol
  port_start        = each.value.port
  port_end          = each.value.port_end
  cidr_ip           = "0.0.0.0/0"
  description       = each.value.desc
}

resource "volcengine_ecs_instance" "gaia" {
  count = var.inst_count

  project_name         = var.project_name
  image_id             = data.volcengine_images.selected.images.0.image_id
  instance_type        = var.inst_type
  instance_name        = "${var.cluster_name}${count.index}"
  description          = "${var.cluster_name}${count.index}"
  instance_charge_type = "PostPaid"
  system_volume_type   = "ESSD_PL0"
  system_volume_size   = 60
  subnet_id            = data.volcengine_subnets.selected.subnets.0.id
  security_group_ids   = [volcengine_security_group.gaiasg.id]
  data_volumes {
    volume_type          = "ESSD_PL0"
    size                 = 100
    delete_with_instance = true
  }
  deployment_set_id = ""

  host_name     = "${var.cluster_name}${count.index}"
  key_pair_name = "fastone-deploy"
  password      = "gaia123ABC"

  user_data = base64encode(local.user_data)
  tags {
    key   = "name"
    value = "gaia"
  }
  tags {
    key   = "author"
    value = "chen"
  }
}

resource "volcengine_eip_address" "gaia" {
  count = var.inst_count

  project_name = var.project_name
  billing_type = "PostPaidByBandwidth"
  bandwidth    = 200
  isp          = "BGP"
  name         = "${var.cluster_name}${count.index}"
  description  = "${var.cluster_name}${count.index}"

  tags {
    key   = "name"
    value = "gaia"
  }
  tags {
    key   = "author"
    value = "chen"
  }
}

resource "volcengine_eip_associate" "gaia" {
  count = var.inst_count

  allocation_id = volcengine_eip_address.gaia["${count.index}"].id
  instance_id   = volcengine_ecs_instance.gaia["${count.index}"].id
  instance_type = "EcsInstance"
}

data "volcengine_eip_addresses" "selected" {
  ids = [volcengine_eip_address.gaia.0.id]
}


resource "null_resource" "setup_client" {
  # Changes to any instance of the cluster requires re-provisioning
  triggers = {
    cluster_instance_ids = "${join(",", volcengine_ecs_instance.gaia.*.id)}"
  }

  connection {
    user = "root"
    host = local.eip
  }

  provisioner "file" {
    source      = "patch.sh"
    destination = "patch.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "bash patch.sh"
    ]
  }
}


output "outs" {
  value = {
    # vpc    = data.volcengine_vpcs.selected.vpcs.0
    # subnet = data.volcengine_subnets.selected.subnets.0
    # sg     = data.volcengine_security_groups.selected
    # sg     = volcengine_security_group.gaiasg
    # sgrule = volcengine_security_group_rule.gaiasgrule
    # images     = data.volcengine_images.selected
    public_ips = volcengine_eip_address.gaia[*]
    eip        = data.volcengine_eip_addresses.selected.addresses.0.eip_address
  }
}

output "created" {
  sensitive = true
  value     = volcengine_ecs_instance.gaia.*
}

locals {

  user_data = <<-EOT
  #!/bin/bash 
    mkdir -p /root/.config/rclone
    cat > /root/.config/rclone/rclone.conf <<EOF
  [fastone]
  type = s3
  provider = Other
  env_auth = true
  access_key_id = ${var.ak}
  secret_access_key = ${var.sk}
  region = cn-shanghai
  endpoint = https://tos-s3-cn-shanghai.volces.com
  force_path_style = false
  disable_http2 = true
  list_version = 2
  EOF

  EOT

  eip = data.volcengine_eip_addresses.selected.addresses.0.eip_address
}
