#!/bin/bash

REGION=eu-west-1

aws cognito-idp list-users --user-pool-id $2 --limit 20 --profile $1 --region $REGION | jq '.["Users"]'  \
	| jq '.[] | select(.Attributes[] | .Value == "'$3'")'  


