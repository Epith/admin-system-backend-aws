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

type Log struct {
	Log_ID      string `json:"log_id"`
	IP          string `json:"ip"`
	Description string `json:"description"`
	UserAgent   string `json:"user_agent"`
	Timestamp   int64  `json:"timestamp"`
	TTL         int64  `json:"ttl"`
}

type ReturnData struct {
	Data []Log  `json:"data"`
	Key  string `json:"key"`
}

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	id := request.QueryStringParameters["id"]
	region := os.Getenv("AWS_REGION")
	LOGS_TABLE := os.Getenv("LOGS_TABLE")

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

	//check if id specified, if yes get single log from dynamo
	if len(id) > 0 {
		res, err := FetchLogByID(id, request, LOGS_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error getting log by ID"),
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

	//check if id specified, if no get all logs from dynamo
	res, err := FetchLogs(request, LOGS_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting logs"),
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

func FetchLogByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*Log, error) {
	//get single log from dynamo
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"log_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("log does not exist")
	}

	item := new(Log)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func FetchLogs(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*ReturnData, error) {
	//get all logs with pagination of limit 100
	key := req.QueryStringParameters["key"]
	lastEvaluatedKey := make(map[string]*dynamodb.AttributeValue)

	item := new([]Log)
	itemWithKey := new(ReturnData)

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int64(int64(100)),
	}

	if len(key) != 0 {
		lastEvaluatedKey["log_id"] = &dynamodb.AttributeValue{
			S: aws.String(key),
		}
		input.ExclusiveStartKey = lastEvaluatedKey
	}

	result, err := dynaClient.Scan(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecord)
	}

	for _, i := range result.Items {
		logItem := new(Log)
		err := dynamodbattribute.UnmarshalMap(i, logItem)
		if err != nil {
			return nil, err
		}
		*item = append(*item, *logItem)
	}

	itemWithKey.Data = *item

	if len(result.LastEvaluatedKey) == 0 {
		return itemWithKey, nil
	}

	itemWithKey.Key = *result.LastEvaluatedKey["log_id"].S

	return itemWithKey, nil
}

func main() {
	lambda.Start(handler)
}
