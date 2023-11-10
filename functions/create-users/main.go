package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
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

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type cognitoUser struct {
	*User
	Password string `json:"password"`
}

type Log struct {
	Log_ID      string    `json:"log_id"`
	User_ID     string    `json:"user_id"`
	Description string    `json:"description"`
	UserAgent   string    `json:"user_agent"`
	Timestamp   time.Time `json:"timestamp"`
	TTL         float64   `json:"ttl"`
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
	if logErr := sendLogs(req, dynaClient); logErr != nil {
		log.Println("Logging err :", logErr)
	}
	var user cognitoUser

	// //marshal body into user
	// if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
	// 	log.Println(err)
	// 	err = errors.New(ErrorInvalidUserData)
	// 	return nil, err
	// }
	// //marshal body into user
	// if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
	// 	err = errors.New(ErrorInvalidUserData)
	// 	return nil, err
	// }

	// //error checks
	// if !IsEmailValid(user.Email) {
	// 	err := errors.New(ErrorInvalidEmail)
	// 	return nil, err
	// }
	// if len(user.FirstName) == 0 {
	// 	return nil, errors.New(ErrorInvalidFirstName)
	// }
	// if len(user.LastName) == 0 {
	// 	err := errors.New(ErrorInvalidLastName)
	// 	return nil, err
	// }
	// user.User_ID = uuid.NewString()

	// //putting user into dynamo
	// av, err := dynamodbattribute.MarshalMap(user.User)

	// if err != nil {
	// 	return nil, errors.New(ErrorCouldNotMarshalItem)
	// }

	// input := &dynamodb.PutItemInput{
	// 	Item:      av,
	// 	TableName: aws.String(tableName),
	// }

	// _, err = dynaClient.PutItem(input)
	// if err != nil {
	// 	return nil, errors.New(ErrorCouldNotDynamoPutItem)
	// }

	// createInput := &cognitoidentityprovider.AdminCreateUserInput{
	// 	DesiredDeliveryMediums: []*string{
	// 		aws.String("EMAIL"),
	// 	},
	// 	ForceAliasCreation: aws.Bool(true),
	// 	UserAttributes: []*cognitoidentityprovider.AttributeType{
	// 		{
	// 			Name:  aws.String("name"),
	// 			Value: aws.String(user.FirstName + user.LastName),
	// 		},
	// 		{
	// 			Name:  aws.String("given_name"),
	// 			Value: aws.String(user.User_ID),
	// 		},
	// 		{
	// 			Name:  aws.String("email_verified"),
	// 			Value: aws.String("True"),
	// 		},
	// 		{
	// 			Name:  aws.String("email"),
	// 			Value: aws.String(user.Email),
	// 		},
	// 		{
	// 			Name:  aws.String("custom:role"),
	// 			Value: aws.String(user.Role),
	// 		},
	// 	},
	// 	UserPoolId: aws.String("ap-southeast-1_jpZj8DWJB"),
	// 	Username:   aws.String(user.User_ID),
	// }

	// _, createErr := cognitoClient.AdminCreateUser(createInput)
	// if createErr != nil {
	// 	log.Println(createErr)
	// 	return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
	// }

	// passwdInput := &cognitoidentityprovider.AdminSetUserPasswordInput{
	// 	Password:   aws.String(user.Password),
	// 	Permanent:  aws.Bool(true),
	// 	Username:   aws.String(user.User_ID),
	// 	UserPoolId: aws.String("ap-southeast-1_jpZj8DWJB"),
	// }

	// _, passwdErr := cognitoClient.AdminSetUserPassword(passwdInput)
	// if passwdErr != nil {
	// 	log.Println(passwdErr)
	// 	return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
	// }

	//EmailVerification(user.Email)
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

func sendLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) error {
	// Calculate the TTL value (one month from now)
	now := time.Now()
	oneWeekFromNow := now.AddDate(0, 0, 7)
	ttlValue := oneWeekFromNow.Unix()

	LOGS_TABLE := os.Getenv("LOGS_TABLE")
	// TTL := os.Getenv("TTL")
	//create log struct
	log := Log{}
	log.Log_ID = uuid.NewString()
	log.User_ID = req.RequestContext.AccountID
	//log.UserAgent = req.Headers["UserAgent"]
	fmt.Println(req.Headers)
	log.TTL = float64(ttlValue)
	log.Description = "test description"
	log.Timestamp = time.Now().UTC()
	fmt.Println(log)
	av, err := dynamodbattribute.MarshalMap(log)

	if err != nil {
		return errors.New("failed to marshal log")
	}
	fmt.Println(av)
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
