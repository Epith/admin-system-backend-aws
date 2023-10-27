# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

output "dynamodb_access_role" {
  value       = aws_iam_role.dynamodb_access_role.arn
  description = "Dynamo access role ARN"
}
