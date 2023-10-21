# Ascenda Loyalty

Ascenda Loyalty program SAM Backend

## 🏗️ Deployment and testing

### Requirements

* [Go](https://go.dev)
* The [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html) for deploying to the cloud
* [Artillery](https://artillery.io/) for load-testing the application

### Commands

You can use the following commands at the root of this repository to build and deploy this project:

```bash
# Compile and prepare Lambda functions
make build

# Deploy the functions on AWS
make deploy

# Clean up program binaries
make clean

# Delete the functions on aws
make delete

# Clean up program binaries
make star-local
```

## Load Test

[Artillery](https://www.artillery.io/) is used to make 300 requests / second for 10 minutes to our API endpoints. You can run this
with the following command:

```bash
make load-test
```
