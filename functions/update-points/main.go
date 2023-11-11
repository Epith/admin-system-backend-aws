package main

import (
	"ascenda/functions/utility"
	"ascenda/types"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
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

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorInvalidUserData         = "invalid user data"
	ErrorInvalidPointsID         = "invalid points id"
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDeleteItem      = "could not delete item"
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

	// Get the parameter value
	paramUser := "USER_TABLE"
	USER_TABLE := utility.GetParameterValue(awsSession, paramUser)

	paramTTL := "TTL"
	TTL := utility.GetParameterValue(awsSession, paramTTL)

	paramLog := "LOGS_TABLE"
	LOGS_TABLE := utility.GetParameterValue(awsSession, paramLog)

	paramPoints := "POINTS_TABLE"
	POINTS_TABLE := utility.GetParameterValue(awsSession, paramPoints)

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
		return nil, errors.New(ErrorInvalidUserData)
	}
	userpoint.User_ID = user_id

	if userpoint.Points_ID == "" {
		err := errors.New(ErrorInvalidPointsID)
		return nil, err
	}

	//checking if userpoint exist
	results, err := FetchUserPoint(user_id, req, tableName, dynaClient)
	if err != nil {
		return nil, errors.New(ErrorInvalidUserData)
	}

	var result = new(types.UserPoint)
	for _, v := range *results {
		if v.Points_ID == userpoint.Points_ID {
			oldPoints = v.Points
			result = &userpoint
		}
	}

	if result.Points_ID != userpoint.Points_ID {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	av, err := dynamodbattribute.MarshalMap(result)
	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	//updating user point in dynamo
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	//logging
	if logErr := sendLogs(req, dynaClient, userTable, logTable, ttl, user_id, oldPoints, userpoint.Points); logErr != nil {
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
		return nil, errors.New(ErrorFailedToFetchRecord)
	}

	if result.Items == nil {
		return nil, errors.New(ErrorUserDoesNotExist)
	}

	item := new([]types.UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func FetchUserByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.User, error) {
	//get single user from dynamo
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("user does not exist")
	}

	item := new(types.User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func main() {
	lambda.Start(handler)
}

func sendLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, userTable string, logTable, ttl string,
	userID string, oldPoints int, newPoints int) error {
	// Calculate the TTL value (one month from now)
	ttlNum, err := strconv.Atoi(ttl)
	if err != nil {
		return errors.New("invalid ttl")
	}

	//get updated user points name
	res, err := FetchUserByID(userID, req, userTable, dynaClient)
	if err != nil {
		log.Println(err)
		return errors.New("failed to get user")
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

	stringOld := strconv.Itoa(oldPoints)
	stringNew := strconv.Itoa(newPoints)

	log.Description = requester + " adjusted points of " + res.FirstName + " " + res.LastName + " from " + stringOld + " to " + stringNew
	log.Timestamp = time.Now().Unix()
	av, err := dynamodbattribute.MarshalMap(log)

	if err != nil {
		return errors.New("failed to marshal log")
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(logTable),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return errors.New("Could not dynamo put")
	}

	return nil
}
