#!/bin/bash

DEMO_RUN_FAST=1
ROOT_DIR=$(dirname $0)/..
. $ROOT_DIR/util.sh

desc "Kubernetes cluster"
run "kubectl get nodes"

desc "Fission installed"
run "kubectl --namespace default get deployment"

clear

desc "NodeJS environment pods"
run "kubectl --namespace fission-function get pod -l environmentName=nodejs"

desc "Function version-1"
run "fission function get --name fn1-v6"

# TODO : create fn1-v7 with status code 400

desc "Function version-2"
run "fission function get --name fn1-v7"

desc "Create a route \(HTTP trigger\) the version-1 of the function with weight 100% and version-2 with weight 0%"
run "fission route create --name route-hello --method GET --url /hello --function fn1-v6 --weight 0 --function fn1-v7 --weight 100"

desc "Create a canary config to gradually increment the weight of version-2 by a step of 20 every 1 minute"
run "fission canary-config create --name canary-1 --funcN fn1-v7 --funcN-1 fn1-v6 --trigger route-hello --increment-step 20 --increment-interval 1m --failure-threshold 10"

sleep 60

# TODO : Find a way to do the below in a for loop

desc "Fire requests to the route"
run "ab -n 30 -c 30 http://$FISSION_ROUTER/hello"
sleep 60

desc "Fire more requests to the route"
run "ab -n 30 -c 30 http://$FISSION_ROUTER/hello"
sleep 60

desc "Fire more requests to the route"
run "ab -n 30 -c 30 http://$FISSION_ROUTER/hello"
sleep 60

desc "Fire more requests to the route"
run "ab -n 30 -c 30 http://$FISSION_ROUTER/hello"
sleep 60

desc "Fire more requests to the route"
run "ab -n 30 -c 30 http://$FISSION_ROUTER/hello"
sleep 60