package main

import (
	"ascenda/functions/utility"
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
	User_ID   string `json:"user_id"`
	Points_ID string `json:"points_id"`
	Points    int    `json:"points"`
}

type ReturnData struct {
	Data     []UserPoint `json:"data"`
	KeyUser  string      `json:"key_user"`
	KeyPoint string      `json:"key_point"`
}

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	user_id := request.QueryStringParameters["id"]
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
	paramPoints := "POINTS_TABLE"
	POINTS_TABLE := utility.GetParameterValue(awsSession, paramPoints)

	//check if user id is specified, if yes call get user point from dynamo func
	if len(user_id) > 0 {
		res, err := FetchUserPoint(user_id, request, POINTS_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error getting point by id"),
			}, nil
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	//check if user id is specified, if no call get all user point from dynamo func
	res, err := FetchUsersPoint(request, POINTS_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting points"),
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

func FetchUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]UserPoint, error) {
	//getting single single user point
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

	if result.Items == nil {
		return nil, errors.New("user point does not exist")
	}

	item := new([]UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func FetchUsersPoint(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*ReturnData, error) {
	//get all user points with pagination of limit 100
	keyUser := req.QueryStringParameters["keyUser"]
	keyPoint := req.QueryStringParameters["keyPoint"]
	lastEvaluatedKey := make(map[string]*dynamodb.AttributeValue)

	item := new([]UserPoint)
	itemWithKey := new(ReturnData)

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int64(int64(100)),
	}

	if len(keyUser) != 0 && len(keyPoint) != 0 {
		lastEvaluatedKey["user_id"] = &dynamodb.AttributeValue{
			S: aws.String(keyUser),
		}
		lastEvaluatedKey["points_id"] = &dynamodb.AttributeValue{
			S: aws.String(keyPoint),
		}
		input.ExclusiveStartKey = lastEvaluatedKey
	}

	result, err := dynaClient.Scan(input)

	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecord)
	}

	for _, i := range result.Items {
		userPoint := new(UserPoint)
		err := dynamodbattribute.UnmarshalMap(i, userPoint)
		if err != nil {
			return nil, err
		}
		*item = append(*item, *userPoint)
	}

	itemWithKey.Data = *item

	if len(result.LastEvaluatedKey) == 0 {
		return itemWithKey, nil
	}

	itemWithKey.KeyUser = *result.LastEvaluatedKey["user_id"].S
	itemWithKey.KeyPoint = *result.LastEvaluatedKey["points_id"].S

	return itemWithKey, nil
}

func main() {
	lambda.Start(handler)
}
