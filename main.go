package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"log"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
)

var sess client.ConfigProvider

func init() {
	// let wkhtmltopdf know where to find our bin file (in the zip)
	os.Setenv("WKHTMLTOPDF_PATH", os.Getenv("LAMBDA_TASK_ROOT"))

	// init aws session for s3 interaction
	sess = session.Must(session.NewSession())
}

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// is request body base64 encoded? why? how? I don't know
	requestBody, _ := base64.StdEncoding.DecodeString(request.Body)
	params := getParametersFromRequestBody(string(requestBody))
	// params["filename"] - preferred filename
	// params["project_name"] - optional folder name to put the document
	// params["margin_top|left|right|bottom"] - manually set margins
	// params["body"] - the html content of the document
	// params["header"] - the header HTML document to appear on every page
	// params["footer"] - the footer HTML document to appear on every page

	// generate object name
	objectName := "generated.pdf"
	if params["filename"] != "" {
		objectName = params["filename"]
	}

	// generate a hash from the POST body to add to the object key
	hasher := md5.New()
	hasher.Write([]byte(request.Body))
	hash := hex.EncodeToString(hasher.Sum(nil))
	objectName = hash + "/" + objectName

	// add it to a project folder
	if params["project_name"] != "" {
		objectName = params["project_name"] + "/" + objectName
	}

	bucketName := os.Getenv("BUCKET_NAME")

	// check to see if the object is already in the bucket
	exists, err := objectExists(objectName, bucketName)
	if err != nil {
		return serverError(err)
	}
	// if the object is already there, return a link to it
	if exists {
		return serverResponse(SignS3ObjectUrl(objectName))
	}

	pdfBytes, err := GeneratePDF(params)
	if err != nil {
		return serverError(err)
	}

	// upload to s3 bucket
	s3Upload := s3manager.NewUploader(sess)
	_, err = s3Upload.Upload(&s3manager.UploadInput{
		Body:   bytes.NewReader(pdfBytes),
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})

	return serverResponse(SignS3ObjectUrl(objectName))
}

func GeneratePDF(params map[string]string) ([]byte, error) {
	pdfGenerator, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return nil, err
	}

	pdfGenerator.NoOutline.Set(true)

	if params["margin_top"] != "" {
		pdfGenerator.MarginTop.Set(stringToUint(params["margin_top"]))
	}

	if params["margin_bottom"] != "" {
		pdfGenerator.MarginBottom.Set(stringToUint(params["margin_bottom"]))
	}

	if params["margin_left"] != "" {
		pdfGenerator.MarginLeft.Set(stringToUint(params["margin_left"]))
	}

	if params["margin_right"] != "" {
		pdfGenerator.MarginRight.Set(stringToUint(params["margin_right"]))
	}

	pageReader := wkhtmltopdf.NewPageReader(strings.NewReader(params["body"]))

	// need to use this option to read local file; reason unknown
	pageReader.LoadErrorHandling.Set("ignore")

	pageReader.Encoding.Set("UTF-8")
	pageReader.DisableSmartShrinking.Set(true)

	// wkhtmltopdf requires that headers and footers are read from files
	// so I have to write them to /tmp first and pass the local filename
	if params["header"] != "" {
		headerFileName := saveHtmlToLocalFile(params["header"])
		pageReader.HeaderHTML.Set(headerFileName)
	}

	if params["footer"] != "" {
		footerFileName := saveHtmlToLocalFile(params["footer"])
		pageReader.FooterHTML.Set(footerFileName)
	}

	pdfGenerator.AddPage(pageReader)

	// create PDF document in internal buffer
	if err := pdfGenerator.Create(); err != nil {
		return nil, err
	}

	// return PDF as bytes array
	return pdfGenerator.Bytes(), nil
}

// save HTML of header or footer to a local tmp file and return the file path
func saveHtmlToLocalFile(html string) string {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "html") // path: /tmp/html[random string]
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
		return ""
	} else {
		_, err = tmpFile.Write([]byte(html))
		if err != nil {
			log.Fatal("Failed to write to temporary file", err)
			return ""
		} else {
			// .html ext required or wkhtmltopdf won't use the file
			tmpFileName := tmpFile.Name()
			tmpNewFileName := tmpFileName + ".html"
			os.Rename(tmpFileName, tmpNewFileName)
			return tmpNewFileName
		}
	}
}

func serverResponse(body string) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body: body,
	}, nil
}

// Add a helper for handling errors. This logs any error to os.Stderr
// and returns a 500 Internal Server Error response that the AWS API
// Gateway understands.
func serverError(err error) (events.APIGatewayProxyResponse, error) {
	log.Println(err.Error())

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body: err.Error(),
		// Body:       http.StatusText(http.StatusInternalServerError),
	}, nil
}

// converts query string in POST body to map
func getParametersFromRequestBody(body string) map[string]string {
	parsedQuery, _ := url.ParseQuery(body)
	// parsedQuery is map[string][]string, but I want a map[string]string
	urlParameters := make(map[string]string)
	for key, _ := range parsedQuery {
		urlParameters[key] = parsedQuery.Get(key)
	}
	return urlParameters
}

func stringToUint(number string) uint {
		numberUint32, _ := strconv.ParseUint(number, 10, 32)
		return uint(numberUint32)
}

// check if object is in bucket
func objectExists(objectName string, bucketName string) (bool, error) {
	service := s3.New(sess)
	response, err := service.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(objectName),
	})

	if err != nil {
		return false, err
	}

	return len(response.Contents) > 0, nil
}
