package main

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"
	"strconv"
	// "strings"
	"time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudfront/sign"
	"github.com/aws/aws-sdk-go/service/s3"
)

/**
 * Step by step instructions at:
 * https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-creating-signed-url-canned-policy.html
 *
 * @param objectKey The path and filename key of the uploaded object with no leading slash
 * @return The signed URL for the object
 *
 * Required environment variables:
 * CLOUDFRONT_PRIVATE_URL - The URL of the distribution sitting in front of the S3 PDF bucket
 * CLOUDFRONT_KEY_PAIR_ID - The public key used to create signed URLs for cloudfront
 * APP_KEYS_BUCKET_NAME - The name of the S3 bucket that contains private keys for signing URLs
 * CLOUDFRONT_PRIVATE_KEY_OBJECT_NAME - The object key name of the private key for signing URLs
 *
 * Final URL format:
 * <cloudfrontUrl>/<objectKey>? \
 * 	Expires=<unixTimestamp>&
 * 	Signature=<hashedAndSignedVersionOfPolicy>&
 * 	Key-Pair-Id=<CloudfrontKeyPairId>
*/
func SignS3ObjectUrl(objectKey string) string {
	baseURL := os.Getenv("CLOUDFRONT_PRIVATE_URL")
	cloudfrontKeyPairId := os.Getenv("CLOUDFRONT_KEY_PAIR_ID")

	// retrieve private key from s3 bucket
	s3Service := s3.New(sess)
	result, err := s3Service.GetObject(&s3.GetObjectInput {
		Bucket: aws.String(os.Getenv("APP_KEYS_BUCKET_NAME")),
		Key: aws.String(os.Getenv("CLOUDFRONT_PRIVATE_KEY_OBJECT_NAME")),
	})
	if err != nil {
		log.Fatalf("Unable to download cloudfront private key from S3")
	}
	// get the object as a string
	buffer := new(bytes.Buffer)
	buffer.ReadFrom(result.Body)
	cloudfrontPrivateKey := buffer.String()

	pem, _ := pem.Decode([]byte(cloudfrontPrivateKey))
	rsaPrivateKey, _ := x509.ParsePKCS1PrivateKey(pem.Bytes)

	// time.Time 15 minutes from now
	expiryTime := time.Now().UTC().Add(time.Minute * time.Duration(15))

	resource := baseURL + "/" + objectKey
	newCannedPolicy := sign.NewCannedPolicy(resource, expiryTime)

	// get b64 encoded signature
	b64Signature, _, err := newCannedPolicy.Sign(rsaPrivateKey)
	if (err != nil) {
		log.Fatalln(err)
	}

	expiryString := strconv.FormatInt(expiryTime.Unix(), 10)

	return resource +
		"?Expires=" + expiryString +
		"&Signature=" + string(b64Signature) +
		"&Key-Pair-Id=" + cloudfrontKeyPairId
}
