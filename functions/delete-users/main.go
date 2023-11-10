package main

import (
	"errors"
	"log"
	"os"
	"strconv"
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

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type Log struct {
	Log_ID      string `json:"log_id"`
	IP          string `json:"ip"`
	Description string `json:"description"`
	UserAgent   string `json:"user_agent"`
	Timestamp   int64  `json:"timestamp"`
	TTL         int64  `json:"ttl"`
}

var (
	ErrorInvalidUUID           = "invalid UUID"
	ErrorCouldNotDeleteItem    = "could not delete item"
	ErrorUserDoesNotExist      = "user does not exist"
	ErrorFailedToFetchRecordID = "failed to fetch record by user id"
	ErrorFailedToUnmarshal     = "failed to unmarshal record from db"
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
		return errors.New(ErrorFailedToFetchRecordID)
	}

	var user User
	if result.Item == nil {
		return errors.New(ErrorUserDoesNotExist)
	}

	err = dynamodbattribute.UnmarshalMap(result.Item, &user)
	if err != nil {
		return errors.New(ErrorFailedToUnmarshal)
	}

	//attempt to delete user in cognito
	cognitoInput := &cognitoidentityprovider.AdminDeleteUserInput{
		Username:   aws.String(id),
		UserPoolId: aws.String("ap-southeast-1_jpZj8DWJB"),
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
		return errors.New(ErrorCouldNotDeleteItem)
	}

	//logging
	if logErr := sendLogs(req, dynaClient, user.FirstName, user.LastName); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}

func sendLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, firstName string, lastName string) error {
	// Calculate the TTL value (one month from now)
	TTL := os.Getenv("TTL")
	ttlNum, err := strconv.Atoi(TTL)
	if err != nil {
		return errors.New("invalid ttl")
	}

	now := time.Now()
	oneWeekFromNow := now.AddDate(0, 0, ttlNum)
	ttlValue := oneWeekFromNow.Unix()

	//requester
	LOGS_TABLE := os.Getenv("LOGS_TABLE")
	requester := req.QueryStringParameters["requester"]

	//create log struct
	log := Log{}
	log.Log_ID = uuid.NewString()
	log.IP = req.Headers["x-forwarded-for"]
	log.UserAgent = req.Headers["user-agent"]
	log.TTL = ttlValue
	log.Description = requester + " deleted user " + firstName + " " + lastName
	log.Timestamp = time.Now().Unix()
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
