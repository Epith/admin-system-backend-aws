package main

import (
	"encoding/json"
	"errors"

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
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	user_id := request.QueryStringParameters["uuid"]
	region := "ap-southeast-1"
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
		}, err
	}
	dynaClient := dynamodb.New(awsSession)
	POINTS_TABLE := "points"

	if len(user_id) > 0 {
		res, err := FetchUserPoint(user_id, POINTS_TABLE, dynaClient)
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
	res, err := FetchUsersPoint(POINTS_TABLE, dynaClient)
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

func FetchUsersPoint(tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]UserPoint, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.Scan(input)
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
