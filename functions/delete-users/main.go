package main

import (
	"ascenda/types"
	"ascenda/utility"
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

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	id := request.QueryStringParameters["id"]
	role := request.QueryStringParameters["role"]
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

	if len(id) > 0 {
		res := DeleteUser(id, role, request, USER_TABLE, LOGS_TABLE, TTL, dynaClient, cognitoClient, USER_POOL_ID)
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

func DeleteUser(id string, role string, req events.APIGatewayProxyRequest, tableName string, logTABLE string, ttl string,
	dynaClient dynamodbiface.DynamoDBAPI, cognitoClient *cognitoidentityprovider.CognitoIdentityProvider, userPoolID string) error {
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
		return errors.New(types.ErrorFailedToFetchRecordID)
	}

	var user types.User
	if result.Item == nil {
		return errors.New(types.ErrorUserDoesNotExist)
	}

	err = dynamodbattribute.UnmarshalMap(result.Item, &user)
	if err != nil {
		return errors.New(types.ErrorFailedToUnmarshal)
	}

	//attempt to delete user in cognito
	cognitoInput := &cognitoidentityprovider.AdminDeleteUserInput{
		Username:   aws.String(id),
		UserPoolId: aws.String(userPoolID),
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
		return errors.New(types.ErrorCouldNotDeleteItem)
	}

	//logging
	if logErr := utility.SendDeleteUserLogs(req, dynaClient, logTABLE, ttl, user.FirstName, user.LastName); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
