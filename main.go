package main

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"log"
	"io/ioutil"
	"os"
	"strings"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
)

func init() {
	// let wkhtmltopdf know where to find our bin file (in the zip)
	os.Setenv("WKHTMLTOPDF_PATH", os.Getenv("LAMBDA_TASK_ROOT"))
}

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// params["body"] - the html content of the document
	// params["header"] - the header HTML document to appear on every page
	// params["footer"] - the footer HTML document to appear on every page
	params := getParametersFromRequestBody(request.Body)
	pdfBytes, err := GeneratePDF(params["body"], params["header"], params["footer"])
	if err != nil {
		return serverError(err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		IsBase64Encoded: true,
		Body: base64.StdEncoding.EncodeToString(pdfBytes),
		Headers: map[string]string{
			"Content-Type": "application/pdf",
		},
	}, nil
}

func GeneratePDF(bodyHtml string, headerHtml string, footerHtml string) ([]byte, error) {
	pdfGenerator, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return nil, err
	}

	pdfGenerator.NoOutline.Set(true)
	pdfGenerator.MarginLeft.Set(0)
	pdfGenerator.MarginRight.Set(0)

	pageReader := wkhtmltopdf.NewPageReader(strings.NewReader(bodyHtml))

	pageReader.Encoding.Set("UTF-8")
	pageReader.DisableSmartShrinking.Set(true)

	// wkhtmltopdf requires that headers and footers are read from files
	// so I have to write them to /tmp first and pass the local filename
	if headerHtml != "" {
		headerFileName := saveHtmlToLocalFile(headerHtml)
		pageReader.HeaderHTML.Set(headerFileName)
	}

	if footerHtml != "" {
		footerFileName := saveHtmlToLocalFile(footerHtml)
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

// save HTML or header or footer to a local tmp file and return the file path
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

// Add a helper for handling errors. This logs any error to os.Stderr
// and returns a 500 Internal Server Error response that the AWS API
// Gateway understands.
func serverError(err error) (events.APIGatewayProxyResponse, error) {
	log.Println(err.Error())

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       http.StatusText(http.StatusInternalServerError),
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
