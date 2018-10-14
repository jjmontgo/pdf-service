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
2.  The service returns signed URL to the rendered PDF document with an expiry.

The setup required for this framework is extensive.  I'll be providing step-by-step instructions on how to set up this service in AWS, and anything not covered will be linked to the AWS documentation.

It is my hope this document will not only help you to generate PDF documents for your own applications, but also serve as a learning tool for AWS services.

## Potential Issues

The approach used to generate PDF documents used by this service has potential drawbacks.

* wkhtmltopdf may not render the provided HTML and CSS according to current standards.  It's up to the developer to adapt their source code to the renderer.
* Image assets aren't included with the source code payload, meaning that wkhtmltopdf must request each image to add to the PDF document.  If a large number of images are included in the source code, or those images are very large, the time needed to download them may cause the Lambda function of the API gateway to timeout.
* I'm not an expert in AWS.  The infrastructure choices made here may not be the best.  Use at your own risk.

## Setup

TODO
