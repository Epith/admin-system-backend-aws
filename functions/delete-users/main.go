package main

import (
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
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
)

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
	ErrorInvalidUUID           = "invalid UUID"
	ErrorCouldNotDeleteItem    = "could not delete item"
	ErrorUserDoesNotExist      = "user does not exist"
	ErrorFailedToFetchRecordID = "failed to fetch record by user id"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	id := request.QueryStringParameters["id"]
	role := request.QueryStringParameters["role"]
	region := os.Getenv("AWS_REGION")
	USER_TABLE := os.Getenv("USER_TABLE")

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
	cognitoClient := cognitoidentityprovider.New(awsSession)

	if len(id) > 0 {
		res := DeleteUser(id, role, request, USER_TABLE, dynaClient, cognitoClient)
		if res != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error deleting User"),
			}, nil
		}
		return events.APIGatewayProxyResponse{
			Body:       "Record successfully deleted",
			StatusCode: 200,
		}, nil
	}

	if logErr := sendLogs(request, 2, 4, "user", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return events.APIGatewayProxyResponse{
		Body:       "User ID missing",
		StatusCode: 404,
	}, nil
}

func DeleteUser(id string, role string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI, cognitoClient *cognitoidentityprovider.CognitoIdentityProvider) error {
	//check if user exist
	checkUser := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(checkUser)
	if err != nil {
		if logErr := sendLogs(req, 3, 4, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return errors.New(ErrorUserDoesNotExist)
	}

	//attempt to delete user in cognito
	cognitoInput := &cognitoidentityprovider.AdminDeleteUserInput{
		Username:   aws.String(id),
		UserPoolId: aws.String("ap-southeast-1_TGeevv7bn"),
	}

	_, cognitoErr := cognitoClient.AdminDeleteUser(cognitoInput)
	if cognitoErr != nil {
		return errors.New(cognitoidentityprovider.ErrCodeInternalErrorException)
	}

	//attempt to delete user in dynamo
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}
	_, err = dynaClient.DeleteItem(input)
	if err != nil {
		if logErr := sendLogs(req, 3, 4, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return errors.New(ErrorCouldNotDeleteItem)
	}

	if logErr := sendLogs(req, 1, 4, "user", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return nil
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
