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
    key    = "global/s3/gaia"
    region = "cn-northwest-1"
  }
}

variable "region" {
  description = "aws region"
  default     = "cn-northwest-1"
}

variable "cluster_name" {
  description = "gaia name"
  default     = "gaiatikv"
}

variable "cluster_meta_name" {
  description = "gaia meta name"
  default     = "gaiameta"
}


variable "cluster_storage_name" {
  description = "gaia storage name"
  default     = "gaiastorage"
}

variable "cluster_client_name" {
  description = "gaia client name"
  default     = "gaiaclient"
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
  default     = "m5d.xlarge"
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
variable "sg_meta_in" {
  description = "sg rule for meta to export to world"
  type = list(object({
    port = number,
    desc = string,
  }))
  default = [
    {
      port = 16686,
      desc = "jadger web ui"
    },
    {
      port = 3000,
      desc = "grafana ui"
    },
    {
      port = 9090,
      desc = "node-exporter"
    },
    {
      port = 9100,
      desc = "prometheus ui"
    },
    {
      port = 4040,
      desc = "pyroscope ui"
    }
  ]
}

data "aws_s3_object" "weed" {
  bucket = "terraform-gaia"
  key    = "bin/weed"
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

resource "aws_security_group" "meta_sg" {
  name        = "allow"
  description = "Allow various traffic for meta"
  vpc_id      = data.aws_vpc.selected.id

  dynamic "ingress" {
    for_each = toset(var.sg_meta_in)
    content {
      description      = ingress.value.desc
      from_port        = ingress.value.port
      to_port          = ingress.value.port
      protocol         = "tcp"
      cidr_blocks      = ["0.0.0.0/0"]
      ipv6_cidr_blocks = ["::/0"]
    }
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  tags = {
    Name = "gaia_meta_sg"
  }
}

# resource "aws_security_group_rule" "meta_sg_rule" {
#   type              = "ingress"
#   from_port         = 16686
#   to_port           = 16686
#   protocol          = "tcp"
#   cidr_blocks       = ["0.0.0.0/0"]
#   ipv6_cidr_blocks  = ["::/0"]
#   security_group_id = aws_security_group.meta_sg.id
# }

resource "aws_instance" "cluster" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.inst_type
  count         = var.inst_count

  key_name = var.key_name


  tags = {
    Name   = "${var.cluster_name}${count.index}"
    Author = var.author
  }
  subnet_id       = data.aws_subnet.selected.id
  security_groups = [data.aws_security_group.selected.id, aws_security_group.meta_sg.id]

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

resource "aws_instance" "meta" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.inst_type
  count         = var.inst_meta_count

  key_name = var.key_name


  tags = {
    Name   = "${var.cluster_meta_name}-${count.index}"
    Author = var.author
  }
  subnet_id       = data.aws_subnet.selected.id
  security_groups = [data.aws_security_group.selected.id, aws_security_group.meta_sg.id]

  #   ebs_block_device  = ""
  root_block_device {
    volume_size = 50
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
      "hostname=${var.cluster_meta_name}${count.index}",
      "echo $hostname | sudo tee /etc/hostname",
      "sudo hostnamectl set-hostname $hostname"
    ]
  }
}

resource "aws_instance" "storage" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.inst_type
  count         = var.inst_storage_count

  key_name = var.key_name

  tags = {
    Name   = "${var.cluster_storage_name}-${count.index}"
    Author = var.author
  }
  subnet_id       = data.aws_subnet.selected.id
  security_groups = [data.aws_security_group.selected.id, aws_security_group.meta_sg.id]

  #   ebs_block_device  = ""
  root_block_device {
    volume_size = 30
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
      "hostname=${var.cluster_storage_name}${count.index}",
      "echo $hostname | sudo tee /etc/hostname",
      "sudo hostnamectl set-hostname $hostname"
    ]
  }
}


resource "aws_instance" "client" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.inst_type
  count         = var.inst_client_count

  key_name = var.key_name

  tags = {
    Name   = "${var.cluster_client_name}-${count.index}"
    Author = var.author
  }
  subnet_id       = data.aws_subnet.selected.id
  security_groups = [data.aws_security_group.selected.id, aws_security_group.meta_sg.id]

  #   ebs_block_device  = ""
  root_block_device {
    volume_size = 50
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
      "hostname=${var.cluster_client_name}${count.index}",
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


resource "null_resource" "setup_topology" {
  # Changes to any instance of the cluster requires re-provisioning
  triggers = {
    cluster_instance_ids = "${join(",", aws_instance.cluster.*.id)}"
  }

  # Bootstrap script can run on any instance of the cluster
  # So we just choose the first in this case
  connection {
    user = "ubuntu"
    host = element(aws_instance.cluster.*.public_ip, 0)
  }

  provisioner "file" {
    content     = local.topology_content
    destination = "topology.yaml"
  }

  provisioner "remote-exec" {
    # Bootstrap script called with private_ip of each node in the cluster
    inline = [
      "curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh",
      "curl -sSL https://d.juicefs.com/install | sh -"
    ]
  }
}

resource "null_resource" "setup_env" {
  # Changes to any instance of the cluster requires re-provisioning
  triggers = {
    cluster_instance_ids = "${join(",", aws_instance.cluster.*.id)}"
  }

  count = length(aws_instance.cluster.*.id)


  # Bootstrap script can run on any instance of the cluster
  # So we just choose the first in this case
  connection {
    user = "ubuntu"
    host = element(aws_instance.cluster.*.public_ip, count.index)
  }

  provisioner "file" {
    content     = local.env_content
    destination = "env.sh"
  }

  provisioner "file" {
    content     = local.gaia_monitor_content
    destination = "monitor.sh"
  }


  provisioner "file" {
    source      = "patch.sh"
    destination = "patch.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "${local.env_content}",
      "bash patch.sh",
      "bash monitor.sh"
    ]
  }
}

resource "null_resource" "setup_meta" {
  # Changes to any instance of the cluster requires re-provisioning
  triggers = {
    cluster_instance_ids = "${join(",", aws_instance.meta.*.id)}"
  }

  count = length(aws_instance.meta.*.id)

  depends_on = [
    null_resource.setup_env
  ]

  # Bootstrap script can run on any instance of the cluster
  # So we just choose the first in this case
  connection {
    user = "ubuntu"
    host = element(aws_instance.meta.*.public_ip, count.index)
  }

  provisioner "file" {
    content     = templatefile("${path.module}/gaia_meta.tftpl", { ip_addrs = local.meta_addrs, host = "${var.cluster_meta_name}${count.index}", otel_addr = aws_instance.cluster[0].private_ip })
    destination = "gaia.sh"
  }


  provisioner "file" {
    source      = "patch.sh"
    destination = "patch.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "echo '${join("\n", formatlist("%v", aws_instance.cluster.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Tikv members:\" }; { print $0 \" ${var.cluster_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.meta.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Meta members:\" }; { print $0 \" ${var.cluster_meta_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.storage.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Storage members:\" }; { print $0 \" ${var.cluster_storage_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.client.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Client members:\" }; { print $0 \" ${var.cluster_client_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "bash gaia.sh"
    ]
  }
}

resource "null_resource" "setup_storage" {
  # Changes to any instance of the cluster requires re-provisioning
  triggers = {
    cluster_instance_ids = "${join(",", aws_instance.storage.*.id)}"
  }

  count = length(aws_instance.storage.*.id)

  depends_on = [
    null_resource.setup_meta
  ]


  # Bootstrap script can run on any instance of the cluster
  # So we just choose the first in this case
  connection {
    user = "ubuntu"
    host = element(aws_instance.storage.*.public_ip, count.index)
  }

  provisioner "file" {
    content     = templatefile("${path.module}/gaia_storage.tftpl", { meta_addrs = local.meta_addrs, host = "${var.cluster_storage_name}${count.index}", otel_addr = aws_instance.cluster[0].private_ip })
    destination = "gaia.sh"
  }


  provisioner "file" {
    source      = "patch.sh"
    destination = "patch.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "echo '${join("\n", formatlist("%v", aws_instance.cluster.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Tikv members:\" }; { print $0 \" ${var.cluster_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.meta.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Meta members:\" }; { print $0 \" ${var.cluster_meta_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.storage.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Storage members:\" }; { print $0 \" ${var.cluster_storage_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.client.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Client members:\" }; { print $0 \" ${var.cluster_client_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "bash gaia.sh"
    ]
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
    content     = local.env_content
    destination = "env.sh"
  }

  provisioner "file" {
    content     = templatefile("${path.module}/gaia_client.tftpl", {})
    destination = "gaia.sh"
  }

  provisioner "file" {
    content     = templatefile("${path.module}/gaia_mount.tftpl", { meta_addrs = local.meta_addrs, host = "${var.cluster_client_name}${count.index}", otel_addr = aws_instance.cluster[0].private_ip })
    destination = "mount.sh"
  }

  provisioner "file" {
    source      = "patch.sh"
    destination = "patch.sh"
  }

  provisioner "file" {
    source      = "fio"
    destination = "fio"
  }

  provisioner "remote-exec" {
    inline = [
      "echo '${join("\n", formatlist("%v", aws_instance.cluster.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Tikv members:\" }; { print $0 \" ${var.cluster_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.meta.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Meta members:\" }; { print $0 \" ${var.cluster_meta_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.storage.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Storage members:\" }; { print $0 \" ${var.cluster_storage_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "echo '${join("\n", formatlist("%v", aws_instance.client.*.private_ip))}' | awk 'BEGIN{ print \"\\n\\n# Client members:\" }; { print $0 \" ${var.cluster_client_name}\" NR-1 }' | sudo tee -a /etc/hosts > /dev/null",
      "bash gaia.sh"
    ]
  }
}


output "public_ips" {
  value = {
    tikv_ips    = aws_instance.cluster.*.public_ip
    meta_ips    = aws_instance.meta.*.public_ip
    storage_ips = aws_instance.storage.*.public_ip
    client_ips  = aws_instance.client.*.public_ip

    all_private_ips = {
      meta_ips    = sort(aws_instance.meta.*.private_ip)
      storage_ips = sort(aws_instance.storage.*.private_ip)
      client_ips  = sort(aws_instance.client.*.private_ip)
    }
  }
}


locals {
  topology_content     = templatefile("${path.module}/init.tftpl", { priamry_addr = aws_instance.cluster[0].private_ip, ip_addrs = aws_instance.cluster.*.private_ip })
  ip_addrs             = join(",", [for c in aws_instance.cluster : "${c.private_ip}:2379"])
  meta_addrs           = join(",", [for index, c in aws_instance.meta : "${var.cluster_meta_name}${index}:26666"])
  storage_addrs        = join(",", [for index, c in aws_instance.storage : "${var.cluster_storage_name}${index}:20001"])
  all_public_ips       = concat(aws_instance.cluster.*.public_ip, aws_instance.meta.*.public_ip, aws_instance.storage.*.public_ip, aws_instance.client.*.public_ip)
  all_node_exporters   = [for index, c in local.all_public_ips : "${c}:9100"]
  env_content          = templatefile("${path.module}/env.tftpl", { ip_addrs = local.ip_addrs })
  gaia_monitor_content = templatefile("${path.module}/gaia_monitor.tftpl", { ip_addrs = local.all_node_exporters })
}
