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

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorInvalidRoleData         = "invalid role data"
	ErrorInvalidRole             = "invalid role"
	ErrorInvalidAccess           = "invalid access"
	ErrorInvalidUUID             = "invalid UUID"
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
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

	//calling create role in dynamo func
	res, err := CreateRole(request, ROLES_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error creating role"),
		}, nil
	}
	body, _ := json.Marshal(res)
	stringBody := string(body)
	return events.APIGatewayProxyResponse{
		Body:       stringBody,
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func CreateRole(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (
	*types.Role,
	error,
) {
	var role types.Role

	//marshal body into role
	if err := json.Unmarshal([]byte(req.Body), &role); err != nil {
		err = errors.New(ErrorInvalidRoleData)
		return nil, err
	}

	//error checks
	if len(role.Role) == 0 {
		err := errors.New(ErrorInvalidRole)
		return nil, err
	}

	//putting role into dynamo
	av, err := dynamodbattribute.MarshalMap(role)

	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(input)
	if err != nil {
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	return &role, nil
}

func main() {
	lambda.Start(handler)
}
