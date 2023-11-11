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
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
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

	// Get the parameter value
	paramUser := "USER_TABLE"
	outputUser, err := utility.GetParameterValue(awsSession, paramUser)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting user table parameter store"),
		}, nil
	}
	USER_TABLE := *outputUser.Parameter.Value

	paramTTL := "TTL"
	outputTTL, err := utility.GetParameterValue(awsSession, paramTTL)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting ttl parameter store"),
		}, nil
	}
	TTL := *outputTTL.Parameter.Value

	paramLog := "LOGS_TABLE"
	outputLogs, err := utility.GetParameterValue(awsSession, paramLog)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting logs table parameter store"),
		}, nil
	}
	LOGS_TABLE := *outputLogs.Parameter.Value

	paramPoints := "POINTS_TABLE"
	outputPoints, err := utility.GetParameterValue(awsSession, paramPoints)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting points table parameter store"),
		}, nil
	}
	POINTS_TABLE := *outputPoints.Parameter.Value

	//checking if user id is specified, if yes then update user in dynamo func
	if len(user_id) > 0 {
		res, err := UpdateUserPoint(user_id, request, POINTS_TABLE, USER_TABLE, LOGS_TABLE, TTL, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string("Error updating point"),
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
		Body:       string("Invalid point data"),
	}, nil

}

func UpdateUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, userTable string, logTable string, ttl string,
	dynaClient dynamodbiface.DynamoDBAPI) (*types.UserPoint, error) {
	var userpoint types.UserPoint
	oldPoints := 0
	//unmarshal body into userpoint struct
	if err := json.Unmarshal([]byte(req.Body), &userpoint); err != nil {
		return nil, errors.New(types.ErrorInvalidUserData)
	}
	userpoint.User_ID = user_id

	if userpoint.Points_ID == "" {
		err := errors.New(types.ErrorInvalidPointsID)
		return nil, err
	}

	//checking if userpoint exist
	results, err := FetchUserPoint(user_id, req, tableName, dynaClient)
	if err != nil {
		return nil, errors.New(types.ErrorInvalidUserData)
	}

	var result = new(types.UserPoint)
	for _, v := range *results {
		if v.Points_ID == userpoint.Points_ID {
			oldPoints = v.Points
			result = &userpoint
		}
	}

	if result.Points_ID != userpoint.Points_ID {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	av, err := dynamodbattribute.MarshalMap(result)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	//updating user point in dynamo
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotDynamoPutItem)
	}

	//logging
	if logErr := utility.SendUpdatePointLogs(req, dynaClient, userTable, logTable, ttl, user_id, oldPoints, userpoint.Points); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return result, nil
}

func FetchUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]types.UserPoint, error) {
	//get single user point
	input := &dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditions: map[string]*dynamodb.Condition{
			"user_id": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(user_id),
					},
				},
			},
		},
	}

	result, err := dynaClient.Query(input)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToFetchRecord)
	}

	if result.Items == nil {
		return nil, errors.New(types.ErrorUserDoesNotExist)
	}

	item := new([]types.UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func main() {
	lambda.Start(handler)
}
