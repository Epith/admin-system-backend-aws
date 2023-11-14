# Ascenda Loyalty Program

Ascenda Loyalty program SAM Backend

## Requirements

* [Go](https://go.dev), the language used for the project
* [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html) for deploying to the cloud
* [Artillery](https://artillery.io/) for load-testing the application
* [Make](https://www.gnu.org/software/make/) for building the Lambda Go handlers on custom runtime

## Initial Setup

```bash
# Set the AWS Region and CF stack name within the Makefile
# STACK_NAME := CF_STACK_NAME
# REGION := PROJECT_REGION

# Initial build of all the Go handlers
make build

# Manual deployment of SAM application
make deploy
```

## Deployment and testing

### Commands

You can use the following commands at the root of this repository to build and deploy this project:

```bash
# Compile and prepare all Lambda functions
make build

# Compile and prepare Lambda functions for specific API
make build-<API folder>

# Deploy the functions on AWS w/ confirmations
make deploy

# Deploy the functions on AWS w/o confirmations
make deploy-auto

# Build and Deploy the all functions on AWS w/o confirmations
make deploy-full-auto

# Go mod tidy the project
make tidy

# Clean up program binaries
make clean

# Delete the SAM Stack on aws
make delete
```

## Load Test

[Artillery](https://www.artillery.io/) is used to make 300 requests / second for 10 minutes to our API endpoints. You can run this
with the following command:

```bash
make load-test
```

