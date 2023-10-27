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
        name = "points_id"
        type = "S"
    }
    attribute {
        name = "user_id"
        type = "S"
    }

    tags = {
        Name = "dynamodb-points"
    }
}

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
        Resource = aws_dynamodb_table.users_table.arn,
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