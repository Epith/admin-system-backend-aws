package utility

import (
	"ascenda/types"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
)

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

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

func SendCreateUserLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, logTABLE string, ttl string,
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
	s := strings.Split(requester, "-")

	//create log struct
	log := types.Log{}
	log.Log_ID = uuid.NewString()
	log.IP = req.Headers["x-forwarded-for"]
	log.UserAgent = req.Headers["user-agent"]
	log.TTL = ttlValue

	if role != "" {
		log.Description = s[0] + " " + s[1] + " enrolled " + role + " " + firstName + " " + lastName
	} else {
		log.Description = s[0] + " " + s[1] + " enrolled user " + firstName + " " + lastName
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

func SendDeleteUserLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, logTABLE string, ttl string,
	firstName string, lastName string) error {
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
	s := strings.Split(requester, "-")

	//create log struct
	log := types.Log{}
	log.Log_ID = uuid.NewString()
	log.IP = req.Headers["x-forwarded-for"]
	log.UserAgent = req.Headers["user-agent"]
	log.TTL = ttlValue
	log.Description = s[0] + " " + s[1] + " deleted user " + firstName + " " + lastName
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

func SendUpdatePointLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, userTable string, logTable, ttl string,
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
	s := strings.Split(requester, "-")

	//create log struct
	log := types.Log{}
	log.Log_ID = uuid.NewString()
	log.IP = req.Headers["x-forwarded-for"]
	log.UserAgent = req.Headers["user-agent"]
	log.TTL = ttlValue

	stringOld := strconv.Itoa(oldPoints)
	stringNew := strconv.Itoa(newPoints)

	log.Description = s[0] + " " + s[1] + " adjusted points of " + res.FirstName + " " + res.LastName + " from " + stringOld + " to " + stringNew
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

func SendUpdateUserLogs(req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI, logTable string, ttl string,
	firstName string, lastName string) error {
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
	s := strings.Split(requester, "-")

	//create log struct
	log := types.Log{}
	log.Log_ID = uuid.NewString()
	log.IP = req.Headers["x-forwarded-for"]
	log.UserAgent = req.Headers["user-agent"]
	log.TTL = ttlValue
	log.Description = s[0] + " " + s[1] + " updated user information for " + firstName + " " + lastName
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
