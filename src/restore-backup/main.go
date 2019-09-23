package main

import (
	"context"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// These types will change based on whatever your function needs to
// consume and return. In this case we are passed a dynamodb.CreateTableInput
// object and return the dynamodb.CreateTableOutput from our call to
// dynamodb.CreateTable.

type (
	// SnsMessage is one message published by SNS
	SnsMessage struct {
		Message           string            `json:Message`
		MessageAttributes map[string]string `json:"MessageAttributes"`
		MessageID         string            `json:"MessageId"`
		Signature         string            `json:"Signature"`
		SignatureVersion  string            `json:"SignatureVersion"`
		SigningCertURL    string            `json:"SigningCertUrl"`
		Subject           string            `json:"Subject"`
		Timestamp         string            `json:"Timestamp"`
		TopicArn          string            `json:"TopicArn"`
		Type              string            `json:"Type"`
		UnsubscribeURL    string            `json:"UnsubscribeUrl"`
	}

	// SnsRecord is one SNS Message with its metadata
	SnsRecord struct {
		EventSource          string     `json:EventSource`
		EventSubscriptionArn string     `json:EventSubscriptionArn`
		EventVersion         string     `json:EventVersion`
		SnsMessage           SnsMessage `json:"Sns"`
	}

	// Input is a collection of SNS Records
	Input struct {
		Records []SnsRecord `json:"Records"`
	}

	// Output in this example is a dynamodb.CreateTableOutput object.
	Output = dynamodb.RestoreTableFromBackupOutput
)

func handler(_ context.Context, input Input) (Output, error) {
	// Create a DynamoDB client named 'svc'
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	// Parse input records to extract the required fields to restore
	// a DynamoDB table
	dynamoDBInput, err := parseSnsInput(input)
	if err != nil {
		return Output{}, err
	}

	// Create the DynamoDB Table
	output, err := svc.RestoreTableFromBackup(&dynamoDBInput)
	if err != nil {
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}

func parseSnsInput(snsInput Input) (dynamodb.RestoreTableFromBackupInput, error) {

	type BackupSnsMessage struct {
		StatusMessage       string
		RecoveryPointArn    string
		BackedUpResourceArn string
		BackupJobID         string
	}

	snsMessage := snsInput.Records[0].SnsMessage.Message

	// Sample Message:
	//  "An AWS Backup job was completed successfully. Recovery point ARN: arn:aws:dynamodb:us-east-1:637093487455:table/MyDynamoDBTable/backup/01568804569000-d3306d76. Backed up Resource ARN : arn:aws:dynamodb:us-east-1:637093487455:table/MyDynamoDBTable. Backup Job Id : 5a772b5a-36d5-4a69-9b18-ed2f5213c659"
	//
	// So we need to extract:
	//   1. StatusMessage (everything before "Recovery point ARN: ")
	//   2. RecoveryPointArn (everything after "Recovery point ARN: " up until the period ".")
	//   3. BackedUpResourceArn (everything after "Backed Up Resource ARN : " up until the period ".")
	//   4. BackupJobId (everything after "Backup Job Id : ")

	// 1. Extract the StatusMessage
	firstSplitKey := "Recovery point ARN: "
	firstSplitString := strings.SplitAfter(snsMessage, firstSplitKey)
	statusMessage := strings.SplitAfter(firstSplitString[0], ".")[0]

	// 2. Extract the RecoveryPointArn from the trailing string
	secondSplitKey := "."
	secondSplitString := strings.SplitAfterN(firstSplitString[1], secondSplitKey, 2)
	recoveryPointArn := strings.Split(secondSplitString[0], ".")[0]

	// 3. Extract the BackedUpResourceArn from the trailing string
	thirdSplitKey := "Backed up Resource ARN : "
	thirdSplitString := strings.SplitAfter(secondSplitString[1], thirdSplitKey)
	tmpString := strings.SplitN(thirdSplitString[1], ".", 2)
	backedUpResourceArn := tmpString[0]

	// 4. Extract the BackupJobId from the remaining string
	fourthSplitKey := "Backup Job Id : "
	fourthSplitString := strings.SplitAfter(tmpString[1], fourthSplitKey)
	backupJobID := fourthSplitString[1]

	backupSnsMessage := BackupSnsMessage{
		StatusMessage: statusMessage,
		RecoveryPointArn: recoveryPointArn,
		BackedUpResourceArn: backedUpResourceArn,
		BackupJobID: backupJobID,
	}

	return dynamodb.RestoreTableFromBackupInput{
		BackupArn: aws.String(backupSnsMessage.RecoveryPointArn),
		TargetTableName: aws.String("MyRestoredDynamoDBTable"),
	}, nil
}
