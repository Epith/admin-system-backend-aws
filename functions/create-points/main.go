package main

import (
	"encoding/json"
	"errors"
	"fmt"
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

type UserPoint struct {
	User_ID   string `json:"user_id"`
	Points_ID string `json:"points_id"`
	Points    int    `json:"points"`
}

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
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

var (
	ErrorInvalidUserData         = "invalid user data"
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	region := os.Getenv("AWS_REGION")
	POINTS_TABLE := os.Getenv("POINTS_TABLE")
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

	//calling create point to dynamo func
	res, err := CreateUserPoint(request, POINTS_TABLE, USER_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error creating point account"),
			Headers:    map[string]string{"content-Type": "application/json"},
		}, err
	}
	body, _ := json.Marshal(res)
	stringBody := string(body)
	return events.APIGatewayProxyResponse{
		Body:       stringBody,
		StatusCode: 200,
		Headers:    map[string]string{"content-Type": "application/json"},
	}, err
}

func CreateUserPoint(req events.APIGatewayProxyRequest, tableName string, userTable string, dynaClient dynamodbiface.DynamoDBAPI) (*UserPoint, error) {
	var userpoint UserPoint

	//marshall body to point struct
	if err := json.Unmarshal([]byte(req.Body), &userpoint); err != nil {
		if logErr := sendLogs(req, 2, 2, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorInvalidUserData)
	}

	//check if user_id is supplied
	if userpoint.User_ID == "" {
		err := errors.New(ErrorInvalidUserData)
		if logErr := sendLogs(req, 2, 2, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}

	//check if user exists
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(userpoint.User_ID),
			},
		},
		TableName: aws.String(userTable),
	}

	result, err := dynaClient.GetItem(input)
	fmt.Print("Errors: ")
	fmt.Println(result.GoString())
	fmt.Println(result.Item)
	if err != nil {
		// if logErr := sendLogs(req, 3, 1, "point", dynaClient, err); logErr != nil {
		// 	log.Println("Logging err :", logErr)
		// }
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}
	// if result.Item == nil || len(result.Item) == 0 {
	// 	return nil, errors.New(ErrorInvalidUserData)
	// }
	item := User{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}
	fmt.Println(item)
	if &item == nil {
		return nil, errors.New(ErrorInvalidUserData)
	}

	userpoint.Points_ID = uuid.NewString()
	userpoint.Points = 0

	//putting into dynamo db
	av, err := dynamodbattribute.MarshalMap(userpoint)

	if err != nil {
		if logErr := sendLogs(req, 3, 1, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	data := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(data)

	if err != nil {
		if logErr := sendLogs(req, 3, 2, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	if logErr := sendLogs(req, 1, 2, "point", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return &userpoint, nil
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
