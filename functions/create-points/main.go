package main

import (
	"ascenda/functions/utility"
	"ascenda/types"
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
	"github.com/google/uuid"
)

var (
	ErrorInvalidUserData         = "invalid user data"
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorUserDoesNotExist        = "user does not exist"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
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
	paramUser := "USER_TABLE"
	USER_TABLE := utility.GetParameterValue(awsSession, paramUser)

	paramPoints := "POINTS_TABLE"
	POINTS_TABLE := utility.GetParameterValue(awsSession, paramPoints)

	//calling create point to dynamo func
	res, err := CreateUserPoint(request, POINTS_TABLE, USER_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error creating point account"),
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

func CreateUserPoint(req events.APIGatewayProxyRequest, tableName string, userTable string, dynaClient dynamodbiface.DynamoDBAPI) (*types.UserPoint, error) {
	var userpoint types.UserPoint

	//marshall body to point struct
	if err := json.Unmarshal([]byte(req.Body), &userpoint); err != nil {
		return nil, errors.New(ErrorInvalidUserData)
	}

	if userpoint.User_ID == "" {
		err := errors.New(ErrorInvalidUserData)
		return nil, err
	}

	//check if user_id is supplied
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(userpoint.User_ID),
			},
		},
		TableName: aws.String(userTable),
	}

	result, err := dynaClient.GetItem(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New(ErrorUserDoesNotExist)
	}

	userpoint.Points_ID = uuid.NewString()
	userpoint.Points = 0

	//putting into dynamo db
	av, err := dynamodbattribute.MarshalMap(userpoint)

	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	data := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(data)

	if err != nil {
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	return &userpoint, nil
}

func main() {
	lambda.Start(handler)
}
