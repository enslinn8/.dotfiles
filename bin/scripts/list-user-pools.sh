#!/bin/bash

aws cognito-idp list-user-pools --max-results 20 --profile $1 --region eu-west-1

