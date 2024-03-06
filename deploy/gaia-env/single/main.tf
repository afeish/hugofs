provider "aws" {
  region     = var.region
  access_key = var.ak
  secret_key = var.sk
}

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.16"
    }
  }

  required_version = ">= 1.2.0"

  backend "s3" {
    bucket = "terraform-gaia"
    key    = "global/s3/single"
    region = "cn-northwest-1"
  }
}

variable "region" {
  description = "aws region"
  default     = "cn-northwest-1"
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
  description = "the aws access_key"
  default     = ""
}

variable "sk" {
  description = "the aws secret_key"
  default     = ""
}

variable "img" {
  description = "aws image regex pattern"
  default     = "ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"
}

variable "key_name" {
  description = "aws key name"
  default     = "fastone-deploy"
}

variable "inst_type" {
  description = "instance type"
  default     = "t3a.xlarge"
}

variable "inst_count" {
  description = "the number of tikv instances"
  default     = 1
}

variable "inst_meta_count" {
  description = "the number of instances"
  default     = 1
}


variable "inst_storage_count" {
  description = "the number of instances"
  default     = 3
}

variable "inst_client_count" {
  description = "the number of instances"
  default     = 1
}

variable "vpc" {
  description = "the name of aws vpc"
  default     = "fastone-demo-region-vpc"
}

variable "subnet" {
  description = "the name of aws subnet"
  default     = "fastone-region-demo-vpc-public-cn-northwest-1a"
}

variable "sg" {
  description = "the name of aws sg"
  default     = "fastone-region-demo-base-gw"
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = [var.img]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  #   owners = ["099720109477"] # Canonical
}

resource "aws_instance" "client" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.inst_type
  count         = var.inst_client_count

  key_name = var.key_name

  tags = {
    Name   = "${var.cluster_name}-${count.index}"
    Author = var.author
  }
  subnet_id       = data.aws_subnet.selected.id
  security_groups = [data.aws_security_group.selected.id]

  #   ebs_block_device  = ""
  root_block_device {
    volume_size = 100
    volume_type = "gp2"
  }

  connection {
    type = "ssh"
    user = "ubuntu"
    host = self.public_ip
  }

  user_data = <<-EOT
  #!/bin/bash 
    mkdir -p /home/ubuntu/.config/rclone
    cat > /home/ubuntu/.config/rclone/rclone.conf <<EOF
  [fastone]
  type = s3
  provider = AWS
  env_auth = true
  region = cn-northwest-1
  access_key_id = ${var.ak}
  secret_access_key = ${var.sk}
  EOF
    sudo chown -R ubuntu:ubuntu /home/ubuntu/.config
  EOT

  provisioner "remote-exec" {
    scripts = [
      "patch.sh"
    ]
  }

  provisioner "remote-exec" {
    inline = [
      "hostname=${var.cluster_name}${count.index}",
      "echo $hostname | sudo tee /etc/hostname",
      "sudo hostnamectl set-hostname $hostname"
    ]
  }
}

data "aws_vpc" "selected" {
  filter {
    name   = "tag:Name"
    values = [var.vpc]
  }
}

data "aws_subnet" "selected" {
  filter {
    name   = "tag:Name"
    values = [var.subnet]
  }
}

data "aws_security_group" "selected" {
  filter {
    name   = "tag:Name"
    values = [var.sg]
  }
}



resource "null_resource" "setup_client" {
  # Changes to any instance of the cluster requires re-provisioning
  triggers = {
    cluster_instance_ids = "${join(",", aws_instance.client.*.id)}"
  }

  count = length(aws_instance.client.*.id)


  # Bootstrap script can run on any instance of the cluster
  # So we just choose the first in this case
  connection {
    user = "ubuntu"
    host = element(aws_instance.client.*.public_ip, count.index)
  }


  provisioner "file" {
    source      = "patch.sh"
    destination = "patch.sh"
  }

}


output "public_ips" {
  value = {
    client_ips = aws_instance.client.*.public_ip
  }
}
