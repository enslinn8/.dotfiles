#!/bin/bash

if [[ -z $1 ]]; then
    echo "User pool id not set";
fi

aws cognito-idp list-users --user-pool-id $1 
