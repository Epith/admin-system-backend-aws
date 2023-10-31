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

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type Log struct {
	Log_ID        string                 `json:"log_id"`
	Severity      int                    `json:"severity"`
	User_ID       string                 `json:"user_id"`
	Action_Type   int                    `json:"action_type"`
	Resource_Type string                 `json:"resource_type"`
	Data          map[string]interface{} `json:"data"`
	Timestamp     time.Time              `json:"timestamp"`
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
	USER_TABLE := os.Getenv("USER_TABLE")

	//setting up dynamo session
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error setting up aws session"),
			Headers:    map[string]string{"content-Type": "application/json"},
		}, err
	}
	dynaClient := dynamodb.New(awsSession)

	//check if id specified, if yes get single user from dynamo
	if len(id) > 0 {
		res, err := FetchUserByID(id, request, USER_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error getting user by id"),
				Headers:    map[string]string{"content-Type": "application/json"},
			}, err
		}
		body, _ := json.Marshal(res)
		stringBody := string(body)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"content-Type": "application/json"},
		}, nil
	}

	//check if id specified, if no get all users from dynamo
	res, err := FetchUsers(request, USER_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting users"),
			Headers:    map[string]string{"content-Type": "application/json"},
		}, err
	}

	body, _ := json.Marshal(res)
	stringBody := string(body)
	return events.APIGatewayProxyResponse{
		Body:       string(stringBody),
		StatusCode: 200,
		Headers:    map[string]string{"content-Type": "application/json"},
	}, nil
}

func FetchUserByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*User, error) {
	//get single user from dynamo
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(input)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	item := new(User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	if logErr := sendLogs(req, 1, 1, "user", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}
	return item, nil
}

func FetchUsers(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]User, error) {
	//get all users
	lastEvaluatedKey := make(map[string]*dynamodb.AttributeValue)
	user := new(User)
	item := new([]User)

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int64(int64(3000)),
	}

	for {

		if len(lastEvaluatedKey) != 0 {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := dynaClient.Scan(input)

		if err != nil {
			if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, errors.New(ErrorFailedToFetchRecord)
		}

		for _, i := range result.Items {
			err := dynamodbattribute.UnmarshalMap(i, user)
			if err != nil {
				if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, err
			}
			*item = append(*item, *user)
		}

		if len(result.LastEvaluatedKey) == 0 {
			if logErr := sendLogs(req, 1, 1, "user", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return item, nil
		}

		lastEvaluatedKey = result.LastEvaluatedKey
	}
}

func main() {
	lambda.Start(handler)
}

func sendLogs(req events.APIGatewayProxyRequest, severity int, action int, resource string, dynaClient dynamodbiface.DynamoDBAPI, err error) error {
	LOGS_TABLE := os.Getenv("LOGS_TABLE")
	//create log struct
	log := Log{}
	data := make(map[string]interface{})
	data["Body"] = RemoveNewlineAndUnnecessaryWhitespace(req.Body)
	data["Query Parameters"] = req.QueryStringParameters["id"]
	data["Error"] = err
	log.Log_ID = uuid.NewString()
	log.Severity = severity
	log.User_ID = req.RequestContext.Identity.User
	log.Action_Type = action
	log.Resource_Type = resource
	log.Data = data
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
