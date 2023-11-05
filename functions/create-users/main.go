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
	Password  string `json:"password"`
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
	ErrorFailedToUnmarshalRecord  = "failed to unmarshal record"
	ErrorFailedToFetchRecord      = "failed to fetch record"
	ErrorFailedToFetchRecordID    = "failed to fetch record by uuid"
	ErrorFailedToFetchRecordEmail = "failed to fetch record by email"
	ErrorInvalidUserData          = "invalid user data"
	ErrorInvalidEmail             = "invalid email"
	ErrorInvalidFirstName         = "invalid first name"
	ErrorInvalidLastName          = "invalid last name"
	ErrorInvalidUUID              = "invalid UUID"
	ErrorCouldNotMarshalItem      = "could not marshal item"
	ErrorCouldNotDeleteItem       = "could not delete item"
	ErrorCouldNotDynamoPutItem    = "could not dynamo put item"
	ErrorUserAlreadyExists        = "user.User already exists"
	ErrorUserDoesNotExist         = "user.User does not exist"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
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

	res, err := CreateUser(request, USER_TABLE, dynaClient, cognitoClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error creating user account"),
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

func CreateUser(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI, cognitoClient *cognitoidentityprovider.CognitoIdentityProvider) (
	*User,
	error,
) {
	var user User

	//marshal body into user
	if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
		log.Println(err)
		err = errors.New(ErrorInvalidUserData)
		if logErr := sendLogs(req, 2, 2, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}

	//error checks
	if !IsEmailValid(user.Email) {
		err := errors.New(ErrorInvalidEmail)
		if logErr := sendLogs(req, 2, 2, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}
	if len(user.FirstName) == 0 {
		err := errors.New(ErrorInvalidFirstName)
		if logErr := sendLogs(req, 2, 2, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorInvalidFirstName)
	}
	if len(user.LastName) == 0 {
		err := errors.New(ErrorInvalidLastName)
		if logErr := sendLogs(req, 2, 2, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}
	user.User_ID = uuid.NewString()

	//putting user into dynamo
	av, err := dynamodbattribute.MarshalMap(user)

	if err != nil {
		if logErr := sendLogs(req, 3, 2, "user", dynaClient, err); logErr != nil {
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
		if logErr := sendLogs(req, 3, 2, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	createInput := &cognitoidentityprovider.AdminCreateUserInput{
		DesiredDeliveryMediums: []*string{
			aws.String("EMAIL"),
		},
		ForceAliasCreation: aws.Bool(true),
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("name"),
				Value: aws.String(user.FirstName + user.LastName),
			},
			{
				Name:  aws.String("email_verified"),
				Value: aws.String("True"),
			},
			{
				Name:  aws.String("email"),
				Value: aws.String(user.Email),
			},
			{
				Name:  aws.String("custom:role"),
				Value: aws.String(user.Role),
			},
		},
		UserPoolId: aws.String("ap-southeast-1_jpZj8DWJB"),
		Username:   aws.String(user.User_ID),
	}

	_, createErr := cognitoClient.AdminCreateUser(createInput)
	if createErr != nil {
		log.Println(createErr)
		if logErr := sendLogs(req, 3, 2, "user", dynaClient, createErr); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
	}

	passwdInput := &cognitoidentityprovider.AdminSetUserPasswordInput{
		Password:   aws.String(user.Password),
		Permanent:  aws.Bool(true),
		Username:   aws.String(user.User_ID),
		UserPoolId: aws.String("ap-southeast-1_jpZj8DWJB"),
	}

	_, passwdErr := cognitoClient.AdminSetUserPassword(passwdInput)
	if passwdErr != nil {
		log.Println(passwdErr)
		if logErr := sendLogs(req, 3, 2, "user", dynaClient, passwdErr); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
	}

	if logErr := sendLogs(req, 1, 2, "user", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return &user, nil
}

func main() {
	lambda.Start(handler)
}

func IsEmailValid(email string) bool {
	var rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]{1,64}@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	if len(email) < 3 || len(email) > 254 || !rxEmail.MatchString(email) {
		return false
	}

	return true
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
