package main

import (
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

type UserPoint struct {
	UserUUID   string `json:"user_id"`
	PointsUUID string `json:"points_id"`
	Points     int    `json:"points"`
}

var (
	ErrorInvalidUserData       = "invalid user data"
	ErrorCouldNotMarshalItem   = "could not marshal item"
	ErrorCouldNotDynamoPutItem = "could not dynamo put item"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	region := os.Getenv("AWS_REGION")
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
		}, err
	}
	dynaClient := dynamodb.New(awsSession)
	POINTS_TABLE := os.Getenv("POINTS_TABLE")

	res, err := CreateUserPoint(request, POINTS_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
		}, err
	}
	stringBody, _ := json.Marshal(res)
	return events.APIGatewayProxyResponse{
		Body:       string(stringBody),
		StatusCode: 200,
	}, err
}

func CreateUserPoint(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*UserPoint, error) {
	var userpoint UserPoint

	if err := json.Unmarshal([]byte(req.Body), &userpoint); err != nil {
		return nil, errors.New(ErrorInvalidUserData)
	}

	if userpoint.UserUUID == "" {
		return nil, errors.New(ErrorInvalidUserData)
	}

	userpoint.PointsUUID = uuid.NewString()
	userpoint.Points = 0

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
