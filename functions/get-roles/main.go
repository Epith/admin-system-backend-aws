package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
)

type Role struct {
	Role   string              `json:"role"`
	Access map[string][]string `json:"access"`
}

type Log struct {
	Log_ID          string      `json:"log_id"`
	Severity        int         `json:"severity"`
	User_ID         string      `json:"user_id"`
	Action_Type     int         `json:"action_type"`
	Resource_Type   string      `json:"resource_type"`
	Body            interface{} `json:"body"`
	QueryParameters interface{} `json:"query_parameters"`
	Error           interface{} `json:"error"`
	Timestamp       time.Time   `json:"timestamp"`
}

type ReturnData struct {
	Data []Role `json:"data"`
	Key  string `json:"key"`
}

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	id := request.QueryStringParameters["role"]
	region := os.Getenv("AWS_REGION")
	ROLES_TABLE := os.Getenv("ROLES_TABLE")

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

func FetchRoleByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*Role, error) {
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
		if logErr := sendLogs(req, 3, 1, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("role does not exist")
	}

	item := new(Role)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	if logErr := sendLogs(req, 1, 1, "role", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}
	return item, nil
}

func FetchRoles(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*ReturnData, error) {
	//get all roles with pagination of limit 100
	key := req.QueryStringParameters["key"]
	lastEvaluatedKey := make(map[string]*dynamodb.AttributeValue)

	item := new([]Role)
	itemWithKey := new(ReturnData)

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
		if logErr := sendLogs(req, 3, 1, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecord)
	}

	for _, i := range result.Items {
		role := new(Role)
		err := dynamodbattribute.UnmarshalMap(i, role)
		if err != nil {
			if logErr := sendLogs(req, 3, 1, "role", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, err
		}
		*item = append(*item, *role)
	}

	itemWithKey.Data = *item

	if len(result.LastEvaluatedKey) == 0 {
		if logErr := sendLogs(req, 1, 1, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return itemWithKey, nil
	}

	itemWithKey.Key = *result.LastEvaluatedKey["role"].S
	if logErr := sendLogs(req, 1, 1, "role", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return itemWithKey, nil
}

func main() {
	lambda.Start(handler)
}

func sendLogs(req events.APIGatewayProxyRequest, severity int, action int, resource string, dynaClient dynamodbiface.DynamoDBAPI, err error) error {
	LOGS_TABLE := os.Getenv("LOGS_TABLE")
	//create log struct
	log := Log{}
	log.Body = RemoveNewlineAndUnnecessaryWhitespace(req.Body)
	log.QueryParameters = req.QueryStringParameters
	log.Error = err
	log.Log_ID = uuid.NewString()
	log.Severity = severity
	log.User_ID = req.RequestContext.Identity.User
	log.Action_Type = action
	log.Resource_Type = resource
	log.Timestamp = time.Now().UTC()

	av, err := dynamodbattribute.MarshalMap(log)

	if err != nil {
		return errors.New("failed to marshal log")
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(LOGS_TABLE),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return errors.New("Could not dynamo put")
	}
	return nil
}

func RemoveNewlineAndUnnecessaryWhitespace(body string) string {
	// Remove newline characters
	body = regexp.MustCompile(`\n|\r`).ReplaceAllString(body, "")

	// Remove unnecessary whitespace
	body = regexp.MustCompile(`\s{2,}|\t`).ReplaceAllString(body, " ")

	// Remove the character `\"`
	body = regexp.MustCompile(`\"`).ReplaceAllString(body, "")

	// Trim the body
	body = strings.TrimSpace(body)

	return body
}
