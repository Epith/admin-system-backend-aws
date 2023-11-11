package main

import (
	"ascenda/functions/utility"
	"encoding/json"
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

var (
	ErrorFailedToFetchRecord   = "failed to fetch record"
	ErrorCouldNotMarshalItem   = "could not marshal item"
	ErrorCouldNotQueryDB       = "could not query db"
	ErrorMakerReqDoesNotExist  = "maker request id does not exist"
	ErrorCouldNotDynamoPutItem = "could not dynamo put item"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	req_id := request.QueryStringParameters["req_id"]
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

	//get parameter value
	paramMaker := "MAKER_TABLE"
	MAKER_TABLE := utility.GetParameterValue(awsSession, paramMaker)

	// get by req id
	if len(req_id) > 0 {
		res, err := FetchMakerRequest(req_id, MAKER_TABLE, request, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string(err.Error()),
			}, nil
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	// get by maker id and status
	makerId := request.QueryStringParameters["maker_id"]
	status := request.QueryStringParameters["status"]
	if len(makerId) > 0 && len(status) > 0 {
		res, err := FetchMakerRequestsByMakerIdAndStatus(makerId, status, MAKER_TABLE, request, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string(err.Error()),
			}, nil
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	} else if len(makerId) > 0 && len(status) == 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Missing status query param"),
		}, nil
	} else if len(makerId) == 0 && len(status) > 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Missing maker_id query param"),
		}, nil
	}
	// get all
	res, err := FetchMakerRequests(MAKER_TABLE, request, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string(err.Error()),
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

func FetchMakerRequest(requestID, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]utility.ReturnMakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("req_id = :req_id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":req_id": {S: aws.String(requestID)},
		},
	}

	result, err := dynaClient.Query(queryInput)
	if err != nil {
		return nil, errors.New(ErrorCouldNotQueryDB)
	}

	if len(result.Items) == 0 {
		return nil, errors.New(ErrorMakerReqDoesNotExist)
	}

	makerRequests := new([]utility.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	return utility.FormatMakerRequest(*makerRequests), nil

}

func FetchMakerRequests(tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) (*utility.ReturnData, error) {
	//get all user points with pagination of limit 100
	keyReq := req.QueryStringParameters["keyReq"]
	keyRole := req.QueryStringParameters["keyRole"]
	lastEvaluatedKey := make(map[string]*dynamodb.AttributeValue)

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int64(int64(100)),
	}

	if len(keyReq) != 0 && len(keyRole) != 0 {
		lastEvaluatedKey["req_id"] = &dynamodb.AttributeValue{
			S: aws.String(keyReq),
		}
		lastEvaluatedKey["checker_role"] = &dynamodb.AttributeValue{
			S: aws.String(keyRole),
		}
		input.ExclusiveStartKey = lastEvaluatedKey
	}

	result, err := dynaClient.Scan(input)

	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecord)
	}
	item := new([]utility.MakerRequest)

	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		return nil, errors.New(utility.ErrorCouldNotUnmarshalItem)
	}

	itemWithKey := new(utility.ReturnData)
	formattedMakerRequests := utility.FormatMakerRequest(*item)
	itemWithKey.Data = formattedMakerRequests

	if len(result.LastEvaluatedKey) == 0 {
		return itemWithKey, nil
	}

	itemWithKey.KeyReq = *result.LastEvaluatedKey["req_id"].S
	itemWithKey.KeyRole = *result.LastEvaluatedKey["checker_role"].S

	return itemWithKey, nil
}

func FetchMakerRequestsByMakerIdAndStatus(makerID, requestStatus, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]utility.ReturnMakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("maker_id-request_status-index"),
		KeyConditionExpression: aws.String("#maker_id = :maker_id AND #request_status = :request_status"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":maker_id":       {S: aws.String(makerID)},
			":request_status": {S: aws.String(requestStatus)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#maker_id":       aws.String("maker_id"),
			"#request_status": aws.String("request_status"),
		},
	}

	result, err := dynaClient.Query(queryInput)
	if err != nil {
		return nil, errors.New(ErrorCouldNotQueryDB)
	}

	makerRequests := new([]utility.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		return nil, errors.New(utility.ErrorCouldNotUnmarshalItem)
	}

	return utility.FormatMakerRequest(*makerRequests), nil
}

func main() {
	lambda.Start(handler)
}
