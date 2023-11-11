package main

import (
	"ascenda/types"
	"ascenda/utility"
	"encoding/json"
	"errors"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorInvalidUserData         = "invalid user data"
	ErrorInvalidUserID           = "invalid points id"
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
	ErrorUserDoesNotExist        = "user.User does not exist"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	user_id := request.QueryStringParameters["id"]
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

	//checking if user id is specified, if yes then update user in dynamo func
	if len(user_id) > 0 {
		res, err := UpdateUser(user_id, request, USER_TABLE, LOGS_TABLE, TTL, dynaClient, cognitoClient, USER_POOL_ID)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error updating user"),
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

	return events.APIGatewayProxyResponse{
		StatusCode: 404,
		Body:       string("Invalid user data"),
	}, nil

}

func UpdateUser(id string, req events.APIGatewayProxyRequest, tableName string, logTable string, ttl string,
	dynaClient dynamodbiface.DynamoDBAPI, cognitoClient *cognitoidentityprovider.CognitoIdentityProvider, userPoolID string) (*types.User, error) {
	var user types.User

	//unmarshal body into user struct
	if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
		return nil, errors.New(ErrorInvalidUserData)
	}
	user.User_ID = id

	if user.User_ID == "" {
		err := errors.New(ErrorInvalidUserID)
		return nil, err
	}

	//checking if user exist
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
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("user does not exist")
	}

	av, err := dynamodbattribute.MarshalMap(user)
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

	//cognito update
	cognitoInput := &cognitoidentityprovider.AdminUpdateUserAttributesInput{
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("name"),
				Value: aws.String(user.FirstName + user.LastName),
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
		Username:   aws.String(id),
	}

	_, cognitoErr := cognitoClient.AdminUpdateUserAttributes(cognitoInput)
	if cognitoErr != nil {
		return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
	}

	//logging
	if logErr := utility.SendUpdateUserLogs(req, dynaClient, logTable, ttl, user.FirstName, user.LastName); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return &user, nil
}

func main() {
	lambda.Start(handler)
}
