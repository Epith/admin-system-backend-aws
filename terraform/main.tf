terraform {
    required_providers {
        aws = {
            source = "hashicorp/aws"
            version = "5.22.0"
        }
    }
}

variable "region" {
  description = "region"
}

provider "aws" {
    region = var.region
}

# DynamoDB Tables
resource "aws_dynamodb_table" "users_table" {
    name = "users"
    billing_mode = "PAY_PER_REQUEST"
    hash_key = "user_id"
    attribute {
        name = "user_id"
        type = "S"
    }

    tags = {
        Name = "dynamodb-users"
    }
}

resource "aws_dynamodb_table" "points_table" {
    name = "points"
    billing_mode = "PAY_PER_REQUEST"
    hash_key = "user_id"
    range_key = "points_id"

    attribute {
        name = "user_id"
        type = "S"
    }

    attribute {
        name = "points_id"
        type = "S"
    }

    tags = {
        Name = "dynamodb-points"
    }
}

resource "aws_dynamodb_table" "makers_table" {
  name           = "requests"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "req_id"
  range_key      = "checker_role"

  attribute {
    name = "req_id"
    type = "S"
  }

  attribute {
    name = "checker_role"
    type = "S"
  }

  attribute {
    name = "maker_id"
    type = "S"
  }

  attribute {
    name = "checker_id"
    type = "S"
  }

  attribute {
    name = "request_status"
    type = "S"
  }

  attribute {
    name = "resource_type"
    type = "S"
  }

  attribute {
    name = "request_data"
    type = "B"
  }

  # Global Secondary Index for maker_id as PK and request_status as SK
  global_secondary_index {
    name = "maker_id-request_status-index"
    hash_key = "maker_id"
    range_key = "request_status"

    projection_type = "ALL"
  }

  # Global Secondary Index for checker_role as PK and request_status as SK
  global_secondary_index {
    name = "checker_role-request_status-index"
    hash_key = "checker_role"
    range_key = "request_status"

    projection_type = "ALL"
  }

  tags = {
    Name = "dynamodb-makers"
  }
  
}

resource "aws_dynamodb_table" "roles_table" {
    name = "roles"
    billing_mode = "PAY_PER_REQUEST"
    hash_key = "role"
    attribute {
        name = "role"
        type = "S"
    }

    tags = {
        Name = "dynamodb-roles"
    }
}

resource "aws_dynamodb_table" "logs_table" {
    name = "logs"
    billing_mode = "PAY_PER_REQUEST"
    hash_key = "log_id"
    attribute {
        name = "log_id"
        type = "S"
    }

    tags = {
        Name = "dynamodb-logs"
    }
}
# Lambda DynamoDB Access Role and Policy
resource "aws_iam_role" "dynamodb_access_role" {
    name = "DynamoDBAccessRole"

    assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
        "Effect": "Allow",
        "Principal": {
            "Service": "lambda.amazonaws.com"
        },
        "Action": "sts:AssumeRole"
        }
    ]
}
EOF
}

resource "aws_iam_policy" "dynamodb_access_policy" {
  name        = "DynamoDBAccessPolicy"
  description = "Policy to allow read and write access to DynamoDB table"

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Action = [
            "dynamodb:GetItem",
            "dynamodb:PutItem",
            "dynamodb:UpdateItem",
            "dynamodb:DeleteItem",
            "dynamodb:Query",
            "dynamodb:Scan",
        ],
        Effect   = "Allow",
        Resource = [
            aws_dynamodb_table.users_table.arn,
            aws_dynamodb_table.points_table.arn,
            aws_dynamodb_table.makers_table.arn,
            aws_dynamodb_table.roles_table.arn,
            aws_dynamodb_table.logs_table.arn,
        ],
      },
    ],
  })
}

resource "aws_iam_role_policy_attachment" "attach_dynamodb_access_policy" {
  policy_arn = aws_iam_policy.dynamodb_access_policy.arn
  role       = aws_iam_role.dynamodb_access_role.name
}

resource "aws_iam_policy" "cloudwatch_logs_policy" {
    name        = "CloudWatchLogsPolicy"
    description = "Policy to allow write access to CloudWatch Logs"
    
    policy = jsonencode({
        Version = "2012-10-17",
        Statement = [
        {
            Action = [
                "logs:CreateLogGroup",
                "logs:CreateLogStream",
                "logs:PutLogEvents",
            ],
            Effect   = "Allow",
            Resource = "*",
        },
        ],
    })
}

resource "aws_iam_role_policy_attachment" "attach_cloudwatch_logs_policy" {
  policy_arn = aws_iam_policy.cloudwatch_logs_policy.arn
  role       = aws_iam_role.dynamodb_access_role.name
}