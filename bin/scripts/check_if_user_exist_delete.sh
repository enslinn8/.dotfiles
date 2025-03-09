#!/bin/bash
REGION=eu-west-1
PROFILE=$1
USERPOOL=$2
EMAIL=$3

id=$(source ~/scripts/list-users.sh $PROFILE $USERPOOL $EMAIL | jq -r '.Username');

if [ -n $id ]; then
	echo "found " $EMAIL;
	source ~/scripts/delete-user.sh $PROFILE $USERPOOL $id;
	echo "deleted " $id;
else
	echo "No item found";
fi

