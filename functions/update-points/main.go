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
)

type UserPoint struct {
	UserUUID   string `json:"user_id"`
	PointsUUID string `json:"points_id"`
	Points     int    `json:"points"`
}

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorInvalidUserData         = "invalid user data"
	ErrorInvalidPointsID         = "invalid points id"
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDeleteItem      = "could not delete item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
	ErrorUserDoesNotExist        = "user.User does not exist"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	user_id := request.QueryStringParameters["uuid"]
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

	if len(user_id) > 0 {
		res, err := UpdateUserPoint(user_id, request, POINTS_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
			}, err
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
		}, nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 404,
	}, errors.New(ErrorInvalidUserData)

}

func UpdateUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*UserPoint, error) {
	var userpoint UserPoint

	if err := json.Unmarshal([]byte(req.Body), &userpoint); err != nil {
		return nil, errors.New(ErrorInvalidUserData)
	}
	userpoint.UserUUID = user_id

	if userpoint.PointsUUID == "" {
		return nil, errors.New(ErrorInvalidPointsID)
	}

	results, err := FetchUserPoint(user_id, tableName, dynaClient)
	if err != nil {
		return nil, errors.New(ErrorInvalidUserData)
	}
	var result = new(UserPoint)
	for _, v := range *results {
		if v.PointsUUID == userpoint.PointsUUID {
			result = &userpoint
		}
	}

	if result.PointsUUID != userpoint.PointsUUID {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	av, err := dynamodbattribute.MarshalMap(result)
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
	return result, nil
}

func FetchUserPoint(user_id string, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]UserPoint, error) {
	input := &dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditions: map[string]*dynamodb.Condition{
			"user_id": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(user_id),
					},
				},
			},
		},
	}

	result, err := dynaClient.Query(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecord)
	}
	item := new([]UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	return item, nil
}

func main() {
	lambda.Start(handler)
}
