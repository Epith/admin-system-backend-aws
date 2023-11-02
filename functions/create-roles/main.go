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

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorInvalidRoleData         = "invalid role data"
	ErrorInvalidRole             = "invalid role"
	ErrorInvalidAccess           = "invalid access"
	ErrorInvalidUUID             = "invalid UUID"
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	region := os.Getenv("AWS_REGION")
	ROLES_TABLE := os.Getenv("ROLES_TABLE")

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

	//calling create role in dynamo func
	res, err := CreateRole(request, ROLES_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error creating role"),
			Headers:    map[string]string{"content-Type": "application/json"},
		}, err
	}
	body, _ := json.Marshal(res)
	stringBody := string(body)
	return events.APIGatewayProxyResponse{
		Body:       stringBody,
		StatusCode: 200,
		Headers:    map[string]string{"content-Type": "application/json"},
	}, nil
}

func CreateRole(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (
	*Role,
	error,
) {
	var role Role

	//marshal body into role
	if err := json.Unmarshal([]byte(req.Body), &role); err != nil {
		err = errors.New(ErrorInvalidRoleData)
		if logErr := sendLogs(req, 2, 2, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}

	//error checks
	if len(role.Role) == 0 {
		err := errors.New(ErrorInvalidRole)
		if logErr := sendLogs(req, 2, 2, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}

	//putting role into dynamo
	av, err := dynamodbattribute.MarshalMap(role)

	if err != nil {
		if logErr := sendLogs(req, 3, 2, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(input)
	if err != nil {
		if logErr := sendLogs(req, 3, 2, "role", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	if logErr := sendLogs(req, 1, 2, "role", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return &role, nil
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
