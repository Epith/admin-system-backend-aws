package main

import (
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

type UserPoint struct {
	User_ID   string `json:"user_id"`
	Points_ID string `json:"points_id"`
	Points    int    `json:"points"`
}

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
	POINTS_TABLE := os.Getenv("POINTS_TABLE")

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

	//checking if user id is specified, if yes then update user in dynamo func
	if len(user_id) > 0 {
		res, err := UpdateUserPoint(user_id, request, POINTS_TABLE, dynaClient)
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

func UpdateUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*UserPoint, error) {
	var userpoint UserPoint
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

	var result = new(UserPoint)
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
	if logErr := sendLogs(req, dynaClient, user_id, oldPoints, userpoint.Points); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return result, nil
}

func FetchUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]UserPoint, error) {
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

	item := new([]UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func FetchUserByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*User, error) {
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

	item := new(User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func main() {
	lambda.Start(handler)
}

func sendLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, userID string, oldPoints int, newPoints int) error {
	// Calculate the TTL value (one month from now)
	TTL := os.Getenv("TTL")
	ttlNum, err := strconv.Atoi(TTL)
	if err != nil {
		return errors.New("invalid ttl")
	}

	//get updated user points name
	USER_TABLE := os.Getenv("USER_TABLE")
	res, err := FetchUserByID(userID, req, USER_TABLE, dynaClient)
	if err != nil {
		log.Println(err)
		return errors.New("failed to get user")
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
		TableName: aws.String(LOGS_TABLE),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return errors.New("Could not dynamo put")
	}

	return nil
}
