# API Gateway via the UI

#### In This Section

- [No Authentication](#none-auth)
- [Basic Authentication](#basic-auth)

Creating API Gateways from the UI is really simple.
Inside your project page, go to the **API Gateways** tab, and click **NEW API Gateway**.

There, you can create an API Gateway with the following parameters:

    - *Name*: The name of the API Gateway.
    - *Description*: A description of the API Gateway.
    - *Host*: The host of the API Gateway.
    - *Path*: The path of the API Gateway.
    - *Authentication Mode*: The authentication mode of the API Gateway.
    - *Function*: The function that will be triggered via the API Gateway. 
                  You can also add a canary function and determine the percentage 
                  of traffic that will be sent to the canary function.

<a id="none-auth"></a>
### No Authentication

![api-gateway](/docs/assets/images/api-gateway-ui.png)

To invoke the function using the api gateway, see [invoking API Gateways](/docs/references/api-gateway/http.md#invoke-none).

<a id="basic-auth"></a>
### With Basic Authentication

![api-gateway-basic-auth](/docs/assets/images/api-gateway-ui-basic-auth.png)

To invoke the function using the api gateway, see [invoking API Gateways with basic authentication](/docs/references/api-gateway/http.md#invoke-basic).