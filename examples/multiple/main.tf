terraform {
  required_providers {
    infrahub = {
      source  = "registry.terraform.io/marcom4rtinez/infrahub"
      version = "1.0"
    }
  }
}

provider "infrahub" {
  api_key         = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxODAyZGZlMS0zYTU5LTE0NjItMzk1ZC1jNTE4ZjQ3ZDEwNjciLCJpYXQiOjE3MzIxNzg2OTYsIm5iZiI6MTczMjE3ODY5NiwiZXhwIjoxNzMyMTgyMjk2LCJmcmVzaCI6ZmFsc2UsInR5cGUiOiJhY2Nlc3MiLCJzZXNzaW9uX2lkIjoiMTgwNDVkMDMtODc1MC01YTIyLTM5NWUtYzUxNWEyMzhhYmJlIiwidXNlcl9jbGFpbXMiOnsicm9sZSI6ImFkbWluIn19.EGvkfPXMB6pip7I3_Fhsax06LY3sXgh1npbsISTpVjw"
  infrahub_server = "10.0.0.1"
}

# data "infrahub_devices" "example" {
# }

# output "devices_example" {
#   value = data.infrahub_devices.example
# }

data "infrahub_device" "fra05-pod1-leaf1" {
  device_name = "fra05-pod1-leaf1"
}

output "device_name_output" {
  value = data.infrahub_device.fra05-pod1-leaf1
}

# output "device_id_output" {
#   value = data.infrahub_device.fra05-pod1-leaf1.id
# }

# output "device_role_output" {
#   value = data.infrahub_device.fra05-pod1-leaf1.role
# }

# resource "infrahub_device" "device_res" {
#   device_name = "fra05-pod6-leaf4"
#   # name        = "fra05-pod1-leaf1"
#   role = "leaf"
#   # id          = "1802e1f2-bc07-e55b-395f-c515fdfc0604"
# }

# output "device_resu" {
#   value = infrahub_device.device_res
# }
