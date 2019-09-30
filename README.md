# AWS Backup DynamoDB Rotator

The AWS Backup DynamoDB Rotator ("the app") restores [Amazon DynamoDB][dynamodb-home] backups to a new timestamped table based on patterns you specify. The app subscribes to an existing [Amazon Simple Notification Service (SNS)][sns-home] topic where [AWS Backup][backup-home] publishes its event notifications. When a BACKUP_JOB_COMPLETE event is received for a DynamoDB table matching a pattern you specify, an [AWS Step Functions][step-functions-home] state machine execution begins that restores the backup to a new table. Optionally, once the restore is complete, an [AWS Systems Manager (SSM)][ssm-home] parameter that you specify is updated with the ARN of the newly-restored table.

## Pre-Requisites

The app requires the following AWS resources to exist before installation:

1. An AWS Backup vault [configured to send notification events to SNS][backup-sns-guide].
1. An SNS topic that receives notifications from the Backup vault.
1. One or more DynamoDB tables configured in Backup that you wish to restore on a recurring basis.
1. A scheduled Backup job in the Backup vault that backs up the DynamoDB tables you wish to restore.

## Parameters

1. `BackupSNSTopicARN` - [Required] The ARN for a previously existing SNS topic to which AWS Backup publishes its notifications. The Step Function will subscribe to this topic and begin execution when a `BACKUP_JOB_COMPLETED` notification is published.
1. `SourcePattern` - [Optional] A regular expression matching the table name - not full ARN - of resources to be restored, e.g., "(?i)-production$" (without the double quotes) for all DynamoDB tables ending with `-production` (case insensitive). If no expression is provided, all DynamoDB tables will be restored. Currently only DynamoDB tables are supported.
1. `ReplacementPattern` - [Optional] A replacement expression used to name the restored resource given in the format, e.g., "-staging" (without the double quotes) to replace the given SourcePatternParameter with "-staging" in the newly restored instance. A date time stamp of the format `-20060102-15-04-05 (-YYYYMMDD-HH-mm-ss)` will be appended to the replacement name in all cases. If no expression is provided, the restored resource will bear the name of the original resource with the date time stamp appended, i.e., use `$0` as the replacement expression.
1. `SSMParameterName` - [Optional] The name and path of an AWS Systems Manager (SSM) Parameter Store parameter to be created or updated with the ARN of the newly restored database, e.g., `/service/staging-database-arn` (without the double quotes). This is useful for automating reporting, staging, and test database rollover. This parameter is optional, and if no value is provided no parameter will be created or updated.

## How it Works

The app subscribes to an existing SNS topic where AWS Backup publishes its event notifications. When a BACKUP_JOB_COMPLETE event is received for a DynamoDB table matching a pattern you specify, an AWS Step Functions state machine execution begins that restores the backup to a new table.

The first Lambda function processes the body of an SNS message sent by AWS Backup to an SNS topic. This lambda function determines whether the resource should be restored using a set of business rules, and if so, initiates an AWS Step Functions state machine using the SDK API call [`SFN.StartExecution`][SFN.StartExecution].

### AWS Step Functions State Machine

The first state machine passes input to the state machine in the following format:

```json
{
    "BackupSnsMessage": {
        "StatusMessage": { "type": "string" },
        "RecoveryPointArn": { "type": "string" },
        "BackedUpResourceArn": { "type": "string" },
        "BackupJobID": { "type": "string" }
    },
    "SourcePattern": { "type": "string" },
    "ReplacementPattern": { "type": "string" },
    "SSMParameterName": { "type": "string" }
}
```

When invoked, the state machine invokes a second Lambda function which initiates the restore using the SDK API call [`DynamoDB.RestoreTableFromBackup`][DynamoDB.RestoreTableFromBackup].

Once the restore is initiated, the state machine checks whether an SSM parameter was defined in the CloudFormation/SAM template. If not, execution completes successfully.

If an SSM parameter was defined, the state machine then sleeps for a pre-determined period before invoking a third Lambda function which checks the status of the restore operation using the SDK API call [`DynamoDB.DescribeTable`][DynamoDB.DescribeTable].

If the restore is not yet complete, the state machine enters a loop of sleeping and checking the status of the restore operation.

Once the restore is complete, the state machine invokes a fourth Lambda function which updates the provided SSM parameter with the ARN of the newly-restored DynamoDB table using the SDK API call [`SSM.PutParameter`][SSM.PutParameter].

Each Lambda function adds its return values to the state in the state machine. On completion, the state is in the following format (top level objects only):

```json
{
    "BackupSnsMessage": {},
    "SourcePattern": "",
    "ReplacementPattern": "",
    "SSMParameterName": "",
    "RestoreTableFromBackupOutput": {},
    "DescribeTableOutput": {},
    "UpdateSSMParameterOutput": {}
}
```

Once completed, we have a newly restored copy of our backup named to match the time of restore.

[backup-home]: https://aws.amazon.com/backup/
[backup-sns-guide]: https://docs.aws.amazon.com/en_pv/aws-backup/latest/devguide/sns-notifications.html
[dynamodb-home]: https://aws.amazon.com/dynamodb/
[sns-home]: https://aws.amazon.com/sns/
[ssm-home]: https://aws.amazon.com/systems-manager/
[step-functions-home]: https://aws.amazon.com/step-functions/

[DynamoDB.DescribeTable]: https://docs.aws.amazon.com/sdk-for-go/api/service/dynamodb/#DynamoDB.DescribeTable
[DynamoDB.RestoreTableFromBackup]: https://docs.aws.amazon.com/sdk-for-go/api/service/dynamodb/#DynamoDB.RestoreTableFromBackup
[SFN.StartExecution]: https://docs.aws.amazon.com/sdk-for-go/api/service/sfn/#SFN.StartExecution
[SSM.PutParameter]: https://docs.aws.amazon.com/sdk-for-go/api/service/ssm/#SSM.PutParameter

[restored-table-image]: images/restored-table.png
[state-machine-image]: images/state-machine-image.png
