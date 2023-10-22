terraform {
    required_providers {
        aws = {
            source = "hashicorp/aws"
            version = "5.22.0"
        }
    }
}

provider "aws" {
  region = "ap-southeast-1"
}

resource "aws_dynamodb_table" "users" {
  name = "users"
  attribute_definitions = [
    { attribute_name = "id", attribute_type = "S" },
  ]
  key_schema = [
    { attribute_name = "id", key_type = "HASH" },
  ]
  provisioned_throughput {
    read_capacity_units = 5
    write_capacity_units = 5
  }
}
