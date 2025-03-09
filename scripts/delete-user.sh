#!/bin/bash

REGION=eu-west-1

aws cognito-idp admin-delete-user \
		--region $REGION \
		--profile $1 \
		--user-pool-id $2\
		--username $3
