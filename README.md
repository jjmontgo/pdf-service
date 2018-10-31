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

### Create the Lambda Function

1.  Choose Services from the main menu.  Search for Lambda and open the Lambda service.
2.  Click the "Create a Function" button.
3.  Leave the default option "Author from scratch."
4.  Enter the name of the Lambda function.  The name of the function should be the same name as the folder directory of your Go project.  In this case, it's `pdf-service`.
5.  Choose Go 1.x for the runtime.
6.  Under Role, choose Create a custom role.
7.  Leave the default IAM Role option "Create a new IAM role" selected.
8.  Enter a descriptive role name.  For example: `pdf-service-role`.
9.  A basic policy document is attached to the role.  This policy allows the role to execute the Lambda function and write CloudWatch logs.  You'll be giving the role access to other services later.  Create the role.
10.  You'll be redirected back to the "Create function" form.  Make sure your new role is selected, and click the "Create function" button.
11.  In the function's Configuration tab, scroll down to the section called "Function code".  In the Handler text box, change the handler name from "hello" to the Go executable file name.  It should be the same name as your function, which is `pdf-service`.
12.  Scroll down to the Basic Settings section and change the Timeout to 5 minutes.
13.  There is a deployment script in the project, `deploy.sh`, which will compile the Go code and deploy it to your Lambda function.  Run it with `sh deploy.sh`.

### Create the S3 Bucket

Now you're going to create an S3 bucket to cache rendered PDF files.  The service caches requests by generating an MD5 digest from the request and using it as part of the object key.

1.  Choose Service from the main menu.  Search for S3 and open the S3 service.
2.  Click the "Create bucket" button.
3.  Enter a meaningful bucket name.  eg. "pdf-service-cache"
4.  Leave all other settings as default.  By default, the bucket is not publically accessible.
5.  Click the bucket's table row in the list of buckets.  An information panel slides in from the right.  Click the button "Copy Bucket ARN" (Amazon Resource Name) and keep it in your buffer.  You'll need the bucket ARN in the following section to give the Lambda function access to the bucket.

### Give the Lambda function access to the bucket

Return to the Lambda function and scroll down to the section "Environment Variables".  Add a new variable called `BUCKET_NAME` with the name of the S3 bucket you just created.  The function will now know which bucket in which to drop the PDF.

The function has to have permission to write to the bucket.  You'll set this permission by adding a permission policy to the role you created when you set up the function.

1.  Choose Services from the main menu.  Search for IAM and open the IAM service.
2.  Choose Roles from the left-hand menu.
3.  Choose the role you created when you set up the Lambda function, eg. "pdf-service-role".
4.  The Permissions tab should be open by default.  Under this tab, click "Attach policies."
5.  Click the "Create Policy" button.  A new tab is opened.
6.  Click Service, and choose S3.
7.  Click Actions, and click the checkbox for All S3 Actions.
8.  Click Resources, and beside `bucket`, click Add ARN.  Paste the Bucket ARN you copied from the previous section, and click Save Changes.  Beside `object`, click Add ARN.  Paste the Bucket ARN again, and for Object name click the Any checkbox.  Click the Add button.
9.  Click the Review Policy button.
10.  In the Name field, choose a highly descriptive name for the policy.  eg. `full-access-pdf-service-cache`
11.  Click the Create Policy button.
12.  Return to the `pdf-service-role` in the previous tab.  You can now add the created policy to the role.
13.  Click the `Filter Policies` link.  Check the `Customer Managed` checkbox.  Choose the policy you just created and click the `Attach Policy` button.

### Give access to the Lambda function through the API Gateway

In order to access the Lambda function from the internet, you'll need to use the Amazon API Gateway.  Here is how to set that up:

1.  Choose services from the main menu.  Search for API Gateway and open the API Gateway service.
2.  If you haven't created an API before, you should see a "Get Started" button.  Click it.
3.  Choose New API.
4.  Enter an API name, eg. "PDF Service API".  Leave everything else as default and click "Create API".
5.  In Resource, click the `Actions` drop-down menu and choose Create Method.
6.  HTML is going to be passed to the API through a POST method.  So choose `POST` and click the checkmark button.
7.  In Integration Type, leave Lambda Function selected.
8.  Check the checkbox beside `Use Lambda Proxy integration`.  Requests will be proxied to Lambda with request details available in the "event" of the function handler.
9.  Leave the default Lambda Region selected.
10.  In the Lambda Function text box, enter the name of the Lambda function.  An autocomplete dropdown will open to let you select the name.
11.  Leave `Use Default Timeout` checked and click the Save button.
12.  A modal opens telling you you're giving the API Gateway permission to invoke your Lambda function.  Click OK.
13.  Under the Actions drop-down menu, choose Deploy API.
14.  Under Deployment Stage, choose New Stage.  We're only going to have one stage.  For stage name, enter "prod" and click Deploy.
15.  You should now see an Invoke URL in the prod Stage Editor.  This is the URL to which you'll POST your HTML to convert to PDF.

### Set up URL signing for the S3 bucket

The Lambda function is going to store the generated PDF in the S3 bucket, and return a signed URL to the client.  The signed URL provides temporary access to the document.  To sign the URL, you'll need to do the following:

* Create a cloudfront distribution as a frontend to the PDF S3 bucket, which is only accessible through signed URLs.
* Get your cloudfront signing keys from your root AWS account.
* Create another S3 bucket to store the cloudfront private key.
* Give the Lambda function access to the S3 bucket with the private key.

#### Create a cloudfront distribution for the PDF S3 bucket

1.  Choose services from the main menu.  Search for CloudFront and open the CloudFront service.
2.  Click the `Create Distribution` button.
3.  Under the Web distribution option, click the `Get Started` button.
4.  In the Origin Domain Name, choose the S3 Bucket, eg. "pdf-service-cache"
5.  For the `Restrict Bucket Access` option, choose `Yes`.
6.  For the `Origin Access Identity` option, choose `Create a New Identity`.
7.  For the `Grant Read Permissions on Bucket` option, choose `Yes, Update Bucket Policy`.
8.  Scroll down to the option `Restrict Viewer Access (Use Signed URLs or Signed Cookies)` and change it to `Yes`.
9.  Leave all other options on their defaults, and click the `Create Distribution` button.
10.  Add the URL of the distribution to your Lambda function with an environment variable called `CLOUDFRONT_PRIVATE_URL`.  The URL is listed under the `Domain Name` column of the cloudfront distribution list.  It uses the format <distributionid>.cloudfront.net.

#### Get your cloudfront signing keys from your root AWS account

1.  You'll need to login to AWS using your root account, which is the only place the keys are available.
2.  Click your name in the top menu and choose `My Security Credentials`.
3.  Dismiss the popup window by clicking `Continue to Security Credentials`.
4.  Open the `CloudFront key pairs` section.
5.  Click the `Create New Keypair` button.  Download both the public and private keys and save them.
6.  Your Lambda function will need to know the public key through an environment variable.  Return to the function and add an environment variable called `CLOUDFRONT_KEY_PAIR_ID`, setting the value to the public key you just downloaded.

#### Create another S3 bucket to store the private key

I never keep secret data in version control.  [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/) might have been a good place for storing keys, but I don't want to pay $0.40 per secret per month.  So I decided to keep the cloudfront private key in a second S3 bucket and fetch it from the Lambda function.

1.  Create a second S3 bucket and name it something appropriate, like "app-private-keys".  Leave all the settings default so the bucket isn't accessible by anyone.
2.  Upload the private key you downloaded in the previous section in the new bucket.  Name it something like "cloudfront-private-key.pem".
3.  Add a new environment variable to the Lambda function called `APP_KEYS_BUCKET_NAME` with the name of the bucket.
4.  Add another environment variable to the Lambda fucntion called `CLOUDFRONT_PRIVATE_KEY_OBJECT_NAME` and set it to the key name in the bucket, "cloudfront-private-key.pem".  Remember to click `Save`.
3.  Now you'll need to give your Lambda function access to the private key.  You can do this by adding a permission policy to the function's role that gives it access to both the bucket and the key in the bucket.

#### Give the Lambda function access to the S3 bucket with the private key.

1.  Click the row representing your new private keys bucket.  Click the `Copy Bucket ARN` button in the panel that slides in from the right.
2.  Under Services, choose IAM again.
3.  Click `Roles` in the left-hand menu.
4.  Open the Lambda function's role (`pdf-service-role`).
5.  Click the `Attach policies` button.
6.  Click the `Create policy` button.
7.  Under Service, choose S3.
8.  Under Actions, check the `Read` access level.
9.  Under Resources, you will first add the bucket ARN you copied earlier.  You will also need to provide the Object.  Click `Add ARN` and paste the bucket ARN again beside `Bucket Name`.  Then enter the object name for your private key, eg. cloudfront-private-key.pem.  Click the `Add` button.
10.  Click the `Review policy` button.
11.  Enter a good name for the policy, such as `access-cloudfront-private-key`.
12.  Click the `Create Policy` button.
13.  Return to the Lambda function role in the previous tab.  
14.  Click the `Filter policies` link and choose `Customer managed`.
15.  Click the Refresh button in the top right-hand corner so that the new policy you created shows up in the list.
16.  Check the new policy and click the `Attach policy` button.

#### Create an IAM user that will have access to the API Gateway through signed URLs.

To create signed URLs for a cloudfront distribution, you use cloudfront keys.  But to create signed URLs to access Amazon resources such as API Gateway, you need to create an IAM user with API keys.

1.  Choose services from the main menu.  Search for IAM and open the IAM Management Console.
2.  Click `Users` in the left hand menu.
3.  Click the `Add user` button.
4.  Give the user the user name `pdf-service`.  Under "Access Type", select `Programmatic Access`.  Click `Next`.
5.  Under "Set Permissions," choose `Attach existing policies directly`.  Click the `Create Policy` button.
6.  Beside "Service," click `Choose a service`.  Search for `ExecuteAPI` and select it.
7.  Under "Access level," check the box for `All ExecuteAPI actions (execute-api:*)`.
8.  Click the `Resources` section and choose `Add ARN`. Unfortunately, this part is a little difficult and will require you to open the AWS console in a separate tab to get the following information.
8.1.  Enter the `Region` your API is in using the right [code](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions).
8.2.  Enter your `Account` number.  To get this number, click the drop-down menu item of your name in the main menu, and click `My Account`.  Your Account number is under "Account Settings" and beside "Account Id".
8.3.  Enter the `Api id`.  Return to your API Gateway and you'll see your Api id in the breadcrumb menu at the end.  eg. APIs > PDF Service (`Api Id is here`).
8.4.  For the `Stage`, enter * for Any stage.
8.5.  For the method, enter `POST`.
9.  Click `Review Policy`.  Enter the name `pdf-service-api-gateway` and click `Create policy`.
10.  Return to the user you were creating in step 4, and add the new policy you created to the user.  You may need to click the Refresh button for the policy to appear in the list.
11.  Click the `Create user` button.
12.  You should now see the "Access key ID" and "Secret access key" for the new user.  Record both of these values.

#### Make the API Gateway only accessible through your IAM user

1.  Choose services from the main menu.  Search for "API Gateway" and open the gateway you set up earlier.
2.  Open the POST method by clicking on `POST`.
3.  Click the `Method Request` link.
4.  Under Settings, beside Authorization, click the edit pencil icon.
5.  Choose `AWS_IAM` and click the checkmark.

#### Generate a signed URL to the AWS Gateway and pass the HTML to be rendered

To retrieve a URL to a rendered PDF, you'll need to do the following in your client code:

1.  Generate an authorization header using your IAM user's public and private API keys:
https://docs.aws.amazon.com/apigateway/api-reference/signing-requests/
2.  POST the HTML to the API Gateway as query parameters in the body of the request, with the authorization header.
3.  The service will return a URL to the generated PDF document.  You can redirect the user to this URL, or anything else you like.

You'll need to implement this in your language of choice.  In my case, the client language was PHP.  So I've included a class that implements the process with [this gist](https://gist.github.com/jjmontgo/2be75d3fb36d680563a6d7d40931d13d).
