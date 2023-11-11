package main

import (
	"ascenda/utility"
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

var (
	ErrorInvalidRole             = "invalid Role"
	ErrorCouldNotDeleteItem      = "could not delete item"
	ErrorRoleDoesNotExist        = "role does not exist"
	ErrorFailedToFetchRecordID   = "failed to fetch record by role"
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	role := request.QueryStringParameters["role"]
	region := os.Getenv("AWS_REGION")

	//setting up dynamo session
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error setting up aws session"),
		}, nil
	}
	dynaClient := dynamodb.New(awsSession)

	// Get the parameter value
	paramRole := "ROLES_TABLE"
	ROLES_TABLE := utility.GetParameterValue(awsSession, paramRole)

	//check if role is supplied, if yes call delete role dynamo func
	if len(role) > 0 {
		err := DeleteRole(role, request, ROLES_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error deleting Role"),
			}, nil
		}
		return events.APIGatewayProxyResponse{
			Body:       "Record successfully deleted",
			StatusCode: 200,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Body:       "Role ID missing",
		StatusCode: 404,
	}, nil
}

func DeleteRole(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) error {
	//checking if role exist
	checkRole := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"role": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(checkRole)
	if err != nil {
		return errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return errors.New(ErrorRoleDoesNotExist)
	}

	//attempt to delete role in dynamo
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"role": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}
	_, err = dynaClient.DeleteItem(input)
	if err != nil {
		return errors.New(ErrorCouldNotDeleteItem)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
