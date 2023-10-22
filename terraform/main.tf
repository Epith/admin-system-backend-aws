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

resource "aws_dynamodb_table" "users_table" {
    name = "users"
    billing_mode = "PAY_PER_REQUEST"
    hash_key = "id"
    attribute {
        name = "id"
        type = "S"
    }

    tags = {
        Name = "dynamodb-users"
    }
}

resource "aws_dynamodb_table" "points_table" {
    name = "points"
    billing_mode = "PAY_PER_REQUEST"
    hash_key = "id"

    attribute {
        name = "id"
        type = "S"
    }
    attribute {
        name = "user_id"
        type = "S"
    }

    global_secondary_index {
        name = "user_id-index"
        hash_key = "user_id"
        projection_type = "ALL"
    }

    tags = {
        Name = "points"
    }
}
