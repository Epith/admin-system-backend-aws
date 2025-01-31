package main

import (
	"ascenda/types"
	"ascenda/utility"
	"encoding/json"
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
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
	outputRoles, err := utility.GetParameterValue(awsSession, paramRole)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting roles table parameter store"),
		}, nil
	}
	ROLES_TABLE := *outputRoles.Parameter.Value

	//checking if role is specified, if yes then update role in dynamo func
	if len(role) > 0 {
		res, err := UpdateRole(role, request, ROLES_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error updating role"),
			}, nil
		}

		body, _ := json.Marshal(res)
		stringBody := string(body)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 404,
		Body:       string("Invalid role data"),
	}, nil

}

func UpdateRole(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.Role, error) {
	var role types.Role

	//unmarshal body into role struct
	if err := json.Unmarshal([]byte(req.Body), &role); err != nil {
		return nil, errors.New(types.ErrorInvalidRoleData)
	}
	role.Role = id

	if role.Role == "" {
		err := errors.New(types.ErrorInvalidRole)
		return nil, err
	}

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
		return nil, errors.New(types.ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New(types.ErrorRoleDoesNotExist)
	}

	av, err := dynamodbattribute.MarshalMap(role)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(input)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotDynamoPutItem)
	}

	return &role, nil
}

func main() {
	lambda.Start(handler)
}
