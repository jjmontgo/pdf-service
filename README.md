# PDF Service

## Overview

This is a remote service which can receive an HTML document from a client application and return a URL to a rendered PDF document.

The service is implemented using the following AWS services:
* A Lambda function written in GoLang
* An API Gateway to provide HTTP access to the Lambda function
* An S3 bucket to cache the rendered PDF document

The actual rendering of the PDF document is handled by [wkhtmltopdf 0.12.4 (with patched qt)](https://wkhtmltopdf.org/).

Security is implemented on two levels:
1.  The client includes an Authorization header when it posts HTML to the service through the API Gateway.
2.  The service returns a signed URL to the rendered PDF document with an expiry.

The setup required for this framework is extensive.  I'll be providing step-by-step instructions on how to set up this service in AWS, and anything not covered will be linked to the AWS documentation.

It is my hope this document will not only help you generate PDF documents for your applications, but also serve as a learning tool for AWS services.

## Potential Issues

The approach to generating PDF documents has potential drawbacks.

* wkhtmltopdf may not render the provided HTML and CSS according to current standards.  It's up to the developer to adapt their source code to the renderer.
* Image assets aren't included with the source code payload, meaning that wkhtmltopdf must request each image to add to the PDF document.  If a large number of images are included in the source code, or those images are very large, the time needed to download them may cause the Lambda function or the API gateway to timeout.
* I'm not an expert in AWS.  The infrastructure choices made here may not be the best.  Use at your own risk.

## Setup Prerequisites

This service is written in the Go programming language.  You should have [Go already installed](https://golang.org/doc/install).  Install all dependencies by running `go get ./...` from your project directory.

Clone this project into your `~/go/src` project directory with:
`git clone https://github.com/jjmontgo/pdf-service`

[Create your AWS Account](https://portal.aws.amazon.com/billing/signup#/start) if you don't already have one.

You'll also need to set up the [AWS command line interface](https://docs.aws.amazon.com/cli/latest/userguide/installing.html) on your system.

## Setup AWS

### Create a Lambda Function

1.  Choose Services from the main menu.  Search for Lambda and open the Lambda service.
2.  Click the `Create a Function` button.
3.  Leave the default option "Author from scratch."
4.  Enter the name of the Lambda function.  The name of the function should be the same name as the folder directory of your Go project.  In this case, it's `pdf-service`.
5.  Choose Go 1.x for the runtime.
6.  Under Role, choose Create a custom role.
7.  Leave the default IAM Role option "Create a new IAM role" selected.
8.  Enter a descriptive role name.  For example: `pdf-service-role`.
9.  A basic policy document is attached to the role.  This policy allows the role to execute the Lambda function and write CloudWatch logs.  You'll be giving the role access to other services later.  Create the role.
10.  You'll be redirected back to the "Create function" form.  Make sure your new role is selected, and click the "Create function" button.


TODO
