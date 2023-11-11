package main

import (
	"ascenda/functions/utility"
	"ascenda/types"
	"encoding/json"
	"errors"
	"log"
	"os"
	"regexp"
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
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/uuid"
)

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

	// Get the parameter value
	paramUser := "USER_TABLE"
	USER_TABLE := utility.GetParameterValue(awsSession, paramUser)

	paramTTL := "TTL"
	TTL := utility.GetParameterValue(awsSession, paramTTL)

	paramLog := "LOGS_TABLE"
	LOGS_TABLE := utility.GetParameterValue(awsSession, paramLog)

	paramUserPool := "USER_POOL_ID"
	USER_POOL_ID := utility.GetParameterValue(awsSession, paramUserPool)

	res, err := CreateUser(request, USER_TABLE, LOGS_TABLE, TTL, dynaClient, cognitoClient, USER_POOL_ID)
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

func CreateUser(req events.APIGatewayProxyRequest, tableName string, logTABLE string, ttl string, dynaClient dynamodbiface.DynamoDBAPI,
	cognitoClient *cognitoidentityprovider.CognitoIdentityProvider, userPoolID string) (
	*types.User,
	error,
) {
	var user types.CognitoUser

	//marshal body into user
	if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
		log.Println(err)
		err = errors.New(ErrorInvalidUserData)
		return nil, err
	}
	//marshal body into user
	if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
		err = errors.New(ErrorInvalidUserData)
		return nil, err
	}

	//error checks
	if !IsEmailValid(user.Email) {
		err := errors.New(ErrorInvalidEmail)
		return nil, err
	}
	if len(user.FirstName) == 0 {
		return nil, errors.New(ErrorInvalidFirstName)
	}
	if len(user.LastName) == 0 {
		err := errors.New(ErrorInvalidLastName)
		return nil, err
	}
	user.User_ID = uuid.NewString()

	//putting user into dynamo
	av, err := dynamodbattribute.MarshalMap(user.User)

	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(input)
	if err != nil {
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
				Name:  aws.String("given_name"),
				Value: aws.String(user.User_ID),
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
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(user.User_ID),
	}

	_, createErr := cognitoClient.AdminCreateUser(createInput)
	if createErr != nil {
		log.Println(createErr)
		return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
	}

	passwdInput := &cognitoidentityprovider.AdminSetUserPasswordInput{
		Password:   aws.String(user.Password),
		Permanent:  aws.Bool(true),
		Username:   aws.String(user.User_ID),
		UserPoolId: aws.String(userPoolID),
	}

	_, passwdErr := cognitoClient.AdminSetUserPassword(passwdInput)
	if passwdErr != nil {
		log.Println(passwdErr)
		return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
	}

	EmailVerification(user.Email)

	//logging
	if logErr := sendLogs(req, dynaClient, logTABLE, ttl, user.FirstName, user.LastName, user.Role); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return user.User, nil
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

func sendLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, logTABLE string, ttl string,
	firstName string, lastName string, role string) error {
	// Calculate the TTL value (one month from now)
	ttlNum, err := strconv.Atoi(ttl)
	if err != nil {
		return errors.New("invalid ttl")
	}

	now := time.Now()
	oneWeekFromNow := now.AddDate(0, 0, ttlNum)
	ttlValue := oneWeekFromNow.Unix()

	//requester
	requester := req.QueryStringParameters["requester"]

	//create log struct
	log := types.Log{}
	log.Log_ID = uuid.NewString()
	log.IP = req.Headers["x-forwarded-for"]
	log.UserAgent = req.Headers["user-agent"]
	log.TTL = ttlValue

	if role != "" {
		log.Description = requester + " enrolled " + role + " " + firstName + " " + lastName
	} else {
		log.Description = requester + " enrolled user " + firstName + " " + lastName
	}
	log.Timestamp = time.Now().Unix()
	av, err := dynamodbattribute.MarshalMap(log)

	if err != nil {
		return errors.New("failed to marshal log")
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(logTABLE),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return errors.New("Could not dynamo put")
	}

	return nil
}

func EmailVerification(emailAddress string) error {
	// Create an SES session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-southeast-1"), // Replace with your desired AWS region
	})
	if err != nil {
		log.Println("Failed to create AWS session", err)
		return err
	}

	sesClient := ses.New(sess)
	_, err = sesClient.VerifyEmailIdentity(&ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(emailAddress),
	})

	if err != nil {
		log.Println("Failed to verify email identity", err)
		return err
	} else {
		log.Println("Verification request sent to", emailAddress)
		return nil
	}
}
