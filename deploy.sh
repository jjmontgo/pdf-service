#!/bin/bash
GOOS=linux go build
zip handler.zip ./${PWD##*/} wkhtmltopdf fonts/*
aws lambda update-function-code --function-name ${PWD##*/} --zip-file fileb://handler.zip
rm handler.zip
