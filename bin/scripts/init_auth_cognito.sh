#!/bin/bash
USER_POOL=eu-west-1_PIY9tLAxV
CLIENT_ID=6qnijij37708722uivpiq2o6am
PROFILE=BAH
REGION=eu-west-1
EMAIL=dylan.luiz@lightcode.co.za
// aws cognito-idp admin-initiate-auth \
//	 --user-pool-id eu-west-1_PIY9tLAxV \
//	 --client-id 6qnijij37708722uivpiq2o6am \
//	 --auth-flow USER_SRP_AUTH \
//	 --auth-parameters USERNAME=dylan.luiz@lightcode.co.za,SRP_A=Hello7321!\
//	 --profile BAH \
//	 --region eu-west-1

aws cognito-idp create-user --user-name johnDoe --email $EMAIL --password MySecretPassword123!
	 --user-pool-id eu-west-1_PIY9tLAxV \
	 --client-id 6qnijij37708722uivpiq2o6am \


