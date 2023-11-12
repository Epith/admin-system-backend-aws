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
	//get variables
	id := request.QueryStringParameters["role"]
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

	//check if role specified, if yes get single role from dynamo
	if len(id) > 0 {
		res, err := FetchRoleByID(id, request, ROLES_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error getting specific role"),
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

	//check if id specified, if no get all roles from dynamo
	res, err := FetchRoles(request, ROLES_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting roless"),
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

func FetchRoleByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.Role, error) {
	//get single role from dynamo
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"role": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(input)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("role does not exist")
	}

	item := new(types.Role)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func FetchRoles(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.ReturnRoleData, error) {
	//get all roles with pagination of limit 100
	key := req.QueryStringParameters["key"]
	lastEvaluatedKey := make(map[string]*dynamodb.AttributeValue)

	item := new([]types.Role)
	itemWithKey := new(types.ReturnRoleData)

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int64(int64(100)),
	}

	if len(key) != 0 {
		lastEvaluatedKey["role"] = &dynamodb.AttributeValue{
			S: aws.String(key),
		}
		input.ExclusiveStartKey = lastEvaluatedKey
	}

	result, err := dynaClient.Scan(input)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToFetchRecord)
	}

	for _, i := range result.Items {
		role := new(types.Role)
		err := dynamodbattribute.UnmarshalMap(i, role)
		if err != nil {
			return nil, err
		}
		*item = append(*item, *role)
	}

	itemWithKey.Data = *item

	if len(result.LastEvaluatedKey) == 0 {
		return itemWithKey, nil
	}

	itemWithKey.Key = *result.LastEvaluatedKey["role"].S

	return itemWithKey, nil
}

func main() {
	lambda.Start(handler)
}
