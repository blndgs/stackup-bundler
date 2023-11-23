#!/bin/bash

set -x

COINBASE=$(curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_coinbase","params":[],"id":1}' http://localhost:8545 | jq -r '.result')
curl -H "Content-Type: application/json" -X POST --data '{"jsonrpc":"2.0","method":"eth_sendTransaction","params":[{"from": "'$COINBASE'", "to":"0x0A7199a96fdf0252E09F76545c1eF2be3692F46b", "value": "0xde0b6b3a7640000"}],"id":1}' http://localhost:8545
curl -H "Content-Type: application/json" -X POST --data '{"jsonrpc":"2.0","method":"eth_sendTransaction","params":[{"from": "'$COINBASE'", "to":"0x3068c2408c01bECde4BcCB9f246b56651BE1d12D", "value": "0xde0b6b3a7640000"}],"id":1}' http://localhost:8545