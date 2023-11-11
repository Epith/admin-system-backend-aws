package main

import (
	"encoding/json"
	"errors"
	"os"

	"ascenda/functions/utility"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

var (
	ErrorCouldNotMarshalItem = "could not marshal item"
	ErrorCouldNotQueryDB     = "could not query db"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	status := request.QueryStringParameters["status"]
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

	//get parameter value
	paramMaker := "MAKER_TABLE"
	MAKER_TABLE := utility.GetParameterValue(awsSession, paramMaker)

	// filter by client role and maker request status
	if len(role) > 0 && len(status) > 0 {
		res, err := FetchMakerRequestsByCheckerRoleAndStatus(role, status, MAKER_TABLE, request, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string(err.Error()),
			}, nil
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 400,
		Body:       string("missing query parameter"),
	}, nil
}

func FetchMakerRequestsByCheckerRoleAndStatus(checker_role, requestStatus, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) (*[]utility.MakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("checker_role-request_status-index"),
		KeyConditionExpression: aws.String("#checker_role = :checker_role AND #request_status = :request_status"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":checker_role":   {S: aws.String(checker_role)},
			":request_status": {S: aws.String(requestStatus)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#checker_role":   aws.String("checker_role"),
			"#request_status": aws.String("request_status"),
		},
	}

	result, err := dynaClient.Query(queryInput)
	if err != nil {
		return nil, errors.New(ErrorCouldNotQueryDB)
	}

	makerRequests := new([]utility.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		return nil, errors.New(utility.ErrorCouldNotUnmarshalItem)
	}

	return makerRequests, nil
}

func main() {
	lambda.Start(handler)
}
