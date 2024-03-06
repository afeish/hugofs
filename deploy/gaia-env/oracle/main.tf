terraform {
  required_providers {
    oci = {
      source = "hashicorp/oci"
    }
  }
}

variable "cluster_name" {
  description = "tikv name"
  default     = "tikv"
}

variable "author" {
  description = "author of the tikv cluster"
  default     = "tikv"
}

variable "region" {
  description = "the region of oracle"
  default     = "ap-tokyo-1"
}

variable "compartment_id" {
  description = "the compartment id of oracle account"
  default     = "ocid1.compartment.oc1..aaaaaaaay2md5b4gc26dksyaqyvvw7znzrrjvk4tlluyejulihzenri5y55a"
}

variable "tenancy_ocid" {
  description = "the ocid of tenancy"
  default     = "ocid1.tenancy.oc1..aaaaaaaa4ffv27e3p2ftlhvhu6afy4rcptdy7mxi5bobl5wnshdm64mixrva"
}

variable "user_ocid" {
  description = "the user's oci id"
  default     = "ocid1.user.oc1..aaaaaaaactquz2a46mv2jpyev6jvkqwh7qvzoiqytyxpopm3jylqqcxpmria"
}

variable "auth" {
  type = object({
    user_ocid   = string
    key         = string
    fingerprint = string
  })
  # default = {
  #   user_ocid   = "ocid1.user.oc1..aaaaaaaactquz2a46mv2jpyev6jvkqwh7qvzoiqytyxpopm3jylqqcxpmria"
  #   fingerprint = "55:7d:43:c4:82:ea:1b:76:71:ce:34:e9:7c:57:e0:3e"
  #   key         = "~/.ssh/oracle.pem"
  # }

}

variable "img" {
  description = "aws image regex pattern"
  default     = "ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"
}

variable "public_key" {
  description = "public key path"
  default     = "~/.ssh/fastone-deploy.pub"
}

variable "inst_type" {
  description = "instance type"
  default     = "VM.Standard2.1"
}

variable "inst_count" {
  description = "the number of instances"
  default     = 3
}

variable "vpc" {
  description = "the name of vpc"
  default     = "fcc-dev"
}

variable "subnet" {
  description = "the name of subnet"
  default     = "公共子网-fcc-dev"
}

variable "sg" {
  description = "the name of sg"
  default     = "dev-kubernetes"
}


provider "oci" {
  region           = var.region
  tenancy_ocid     = var.tenancy_ocid
  user_ocid        = var.auth.user_ocid
  fingerprint      = var.auth.fingerprint
  private_key_path = var.auth.key
  # auth                = "SecurityToken"
  # config_file_profile = "learn-terraform"
}

data "oci_identity_availability_domains" "ads" {
  compartment_id = var.compartment_id
}

output "availability_domains" {
  value = data.oci_identity_availability_domains.ads.availability_domains
}

data "oci_core_vcns" "test_vcns" {
  #Required
  compartment_id = var.compartment_id

  display_name = var.vpc
  state        = "AVAILABLE"
}

output "vcn_out" {
  value = data.oci_core_vcns.test_vcns.virtual_networks
}

data "oci_core_subnets" "test_subnets" {
  #Required
  compartment_id = var.compartment_id

  #Optional
  display_name = var.subnet
  # state        = var.subnet_state
  vcn_id = data.oci_core_vcns.test_vcns.virtual_networks[0].id
}


output "subnet_out" {
  value = data.oci_core_subnets.test_subnets
}

# https://gmusumeci.medium.com/how-to-get-the-latest-os-image-in-oracle-cloud-infrastructure-using-terraform-f53823223968
# get latest Ubuntu Linux 20.04 image
data "oci_core_images" "ubuntu-20-04" {
  compartment_id   = var.compartment_id
  operating_system = "Canonical Ubuntu"
  filter {
    name   = "display_name"
    values = ["^Canonical-Ubuntu-20.04-([\\.0-9-]+)$"]
    regex  = true
  }
}
output "ubuntu-20-04-latest-name" {
  value = data.oci_core_images.ubuntu-20-04.images.0.display_name
}
output "ubuntu-20-04-latest-id" {
  value = data.oci_core_images.ubuntu-20-04.images.0.id
}


data "oci_core_network_security_groups" "k8s_sg" {

  #Optional
  compartment_id = var.compartment_id
  display_name   = var.sg
  vcn_id         = data.oci_core_vcns.test_vcns.virtual_networks[0].id
}

output "sg" {
  value = data.oci_core_network_security_groups.k8s_sg.network_security_groups[0].id
}

data "oci_core_shapes" "test_shapes" {
  #Required
  compartment_id = var.compartment_id

  #Optional
  availability_domain = local.az
  image_id            = data.oci_core_images.ubuntu-20-04.images.0.id
}

#output "shapes" {
#  value = data.oci_core_shapes.test_shapes
#}

resource "oci_core_instance" "ubuntu_instance" {
  # Required
  availability_domain = data.oci_identity_availability_domains.ads.availability_domains[0].name
  compartment_id      = var.compartment_id
  shape               = var.inst_type
  source_details {
    source_id               = data.oci_core_images.ubuntu-20-04.images.0.id
    source_type             = "image"
    boot_volume_size_in_gbs = 200
  }

  count = var.inst_count

  # Optional
  display_name = "${var.cluster_name}-${count.index}"
  create_vnic_details {
    assign_public_ip = true
    subnet_id        = data.oci_core_subnets.test_subnets.subnets[0].id
    nsg_ids          = [data.oci_core_network_security_groups.k8s_sg.network_security_groups[0].id]

  }
  metadata = {
    ssh_authorized_keys = file(var.public_key)
  }
  preserve_boot_volume = false

  connection {
    type = "ssh"
    user = "ubuntu"
    host = self.public_ip
  }


  provisioner "remote-exec" {
    scripts = [
      "patch.sh",
    ]
  }
}

resource "null_resource" "setup_topology" {
  # Changes to any instance of the cluster requires re-provisioning
  triggers = {
    cluster_instance_ids = "${join(",", oci_core_instance.ubuntu_instance.*.id)}"
  }

  # Bootstrap script can run on any instance of the cluster
  # So we just choose the first in this case
  connection {
    user = "ubuntu"
    host = element(oci_core_instance.ubuntu_instance.*.public_ip, 0)
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
    cluster_instance_ids = "${join(",", oci_core_instance.ubuntu_instance.*.id)}"
  }

  count = length(oci_core_instance.ubuntu_instance.*.id)


  # Bootstrap script can run on any instance of the cluster
  # So we just choose the first in this case
  connection {
    user = "ubuntu"
    host = element(oci_core_instance.ubuntu_instance.*.public_ip, count.index)
  }

  provisioner "file" {
    content     = local.env_content
    destination = "env.sh"
  }


  provisioner "file" {
    source      = "patch.sh"
    destination = "patch.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "${local.env_content}",
      "bash patch.sh"
    ]
  }
}



output "public_ips" {
  value = {
    tikv_ips = oci_core_instance.ubuntu_instance.*.public_ip
  }
}


locals {
  topology_content = templatefile("${path.module}/init.tftpl", { priamry_addr = oci_core_instance.ubuntu_instance[0].private_ip, ip_addrs = oci_core_instance.ubuntu_instance.*.private_ip })
  ip_addrs         = join(",", [for c in oci_core_instance.ubuntu_instance : "${c.private_ip}:2379"])
  env_content      = templatefile("${path.module}/env.tftpl", { ip_addrs = local.ip_addrs })
  az               = data.oci_identity_availability_domains.ads.availability_domains[0].name
}

