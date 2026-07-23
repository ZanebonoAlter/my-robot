# Quick Start - Overview
> 原文链接: https://docs.z.ai

---

Tired of limits? GLM Coding Plan — monthly access to world-class models, compatible with top coding tools like Claude Code and Cline. All from just $18/month. [Try it now →](https://z.ai/subscribe?utm_campaign=Platform_Ops&_channel_track_key=DaprgHIc)

##

[​

](#getting-started)

Getting Started

1

Get API Key

-   Access [Z.AI Open Platform](https://z.ai/model-api), Register or Login.
-   Access [Billing Page](https://z.ai/manage-apikey/billing) to top up if needed.
-   Create an API Key in the [API Keys](https://z.ai/manage-apikey/apikey-list) management page.
-   Copy your API Key for use.

    [

    ](https://z.ai/model-api)

    [

    ## Z.AI Open Platform

    ](https://z.ai/model-api)

    [Access](https://z.ai/model-api) [Z.AI Open Platform](https://z.ai/model-api), Register or Login.

    [

    ](https://z.ai/manage-apikey/apikey-list)

    [

    ## API Keys Management

    ](https://z.ai/manage-apikey/apikey-list)

    [Create an API Key in the](https://z.ai/manage-apikey/apikey-list) [API Keys](https://z.ai/manage-apikey/apikey-list) management page.

2

Choose Model

> The platform offers multiple models, and you can select the appropriate model based on your needs. For detailed model introductions, please refer to the [Models & Agents](https://docs.z.ai/guides/overview/pricing).

[

## GLM-5.2

Zai’s strongest coding model to date, supporting up to 1M context, with the new reasoning\_effort parameter for controlling reasoning depth.

](https://docs.z.ai/guides/llm/glm-5.2)

[

## GLM-5V-Turbo

Multimodal Coding model, specializing in visual programming.

](https://docs.z.ai/guides/vlm/glm-5v-turbo)

[

## GLM-Image

Supports text-to-image generation, achieving open-source state-of-the-art (SOTA) in complex scenarios

](https://docs.z.ai/guides/image/glm-image)

[

## CogVideoX-3

New frame generation capabilities that significantly improve image stability and clarity

](https://docs.z.ai/guides/video/cogvideox-3)

3

Choose the Calling Method

Our platform provides various development approaches; you can select the best fit for your project needs and tech stack.

[

## HTTP API

Standard RESTful API, compatible with all programming languages.

](https://docs.z.ai/guides/develop/http/introduction)

[

## Z.AI Python SDK

Official Python SDK, featuring full type hints and async support.

](https://docs.z.ai/guides/develop/python/introduction)

[

## Z.AI Java SDK

Official Java SDK, designed for high concurrency and availability.

](https://docs.z.ai/guides/develop/java/introduction)

[

## OpenAI Python SDK

OpenAI SDK Compatibility, quickly migrating from OpenAI.

](https://docs.z.ai/guides/develop/openai/python)

[

## API Reference

Complete API documentation with parameter descriptions.

](https://docs.z.ai/api-reference)

4

Make API Call

After preparing your `API Key` and selecting a model, you can start making API calls. Here are examples using `curl`, `Python SDK`, and `Java SDK`:

When using GLM Coding Plan, please follow the [tutorial](https://docs.z.ai/devpack/quick-start) to configure your dedicated endpoint.

-   cURL

-   Official Python SDK

-   Official Java SDK

-   OpenAI Python SDK

-   OpenAI NodeJs SDK

-   OpenAI Java SDK

```
curl -X POST "https://api.z.ai/api/paas/v4/chat/completions" \
-H "Content-Type: application/json" \
-H "Accept-Language: en-US,en" \
-H "Authorization: Bearer YOUR_API_KEY" \
-d '{
    "model": "glm-5.2",
    "messages": [
        {
            "role": "system",
            "content": "You are a helpful AI assistant."
        },
        {
            "role": "user",
            "content": "Hello, please introduce yourself."
        }
    ]
}'
```

**Install SDK**

```
# Install latest version
pip install zai-sdk

# Or specify version
pip install zai-sdk==0.2.3
```

**Verify Installation**

```
import zai
print(zai.__version__)
```

**Usage Example**

```
from zai import ZaiClient

# Initialize client
client = ZaiClient(api_key="YOUR_API_KEY")

# Create chat completion request
response = client.chat.completions.create(
    model="glm-5.2",
    messages=[
        {
            "role": "system",
            "content": "You are a helpful AI assistant."
        },
        {
            "role": "user",
            "content": "Hello, please introduce yourself."
        }
    ]
)

# Get response
print(response.choices[0].message.content)
```

**Install SDK****Maven**

```
<dependency>
    <groupId>ai.z.openapi</groupId>
    <artifactId>zai-sdk</artifactId>
    <version>0.3.5</version>
</dependency>
```

**Gradle (Groovy)**

```
implementation 'ai.z.openapi:zai-sdk:0.3.5'
```

**Usage Example**

```
import ai.z.openapi.ZaiClient;
import ai.z.openapi.service.model.*;
import java.util.Arrays;

public class QuickStart {
    public static void main(String[] args) {
        // Initialize client
        ZaiClient client = ZaiClient.builder().ofZAI()
            .apiKey("YOUR_API_KEY")
            .build();

        // Create chat completion request
        ChatCompletionCreateParams request = ChatCompletionCreateParams.builder()
            .model("glm-5.2")
            .messages(Arrays.asList(
                ChatMessage.builder()
                    .role(ChatMessageRole.USER.value())
                    .content("Hello, who are you?")
                    .build()
            ))
            .stream(false)
            .build();

        // Send request
        ChatCompletionResponse response = client.chat().createChatCompletion(request);

        // Get response
        System.out.println(response.getData().getChoices().get(0).getMessage().getContent());
    }
}
```

**Install SDK**

```
# Install or upgrade to latest version
pip install --upgrade 'openai>=1.0'
```

**Verify Installation**

```
python -c "import openai; print(openai.__version__)"
```

**Usage Example**

```
from openai import OpenAI

client = OpenAI(
    api_key="your-Z.AI-api-key",
    base_url="https://api.z.ai/api/paas/v4/"
)

completion = client.chat.completions.create(
    model="glm-5.2",
    messages=[
        {"role": "system", "content": "You are a smart and creative novelist"},
        {"role": "user", "content": "Please write a short fairy tale story as a fairy tale master"}
    ]
)

print(completion.choices[0].message.content)
```

**Install SDK**

```
# Install or upgrade to latest version
npm install openai

# Or using yarn
yarn add openai
```

**Usage Example**

```
import OpenAI from "openai";

const client = new OpenAI({
    apiKey: "your-Z.AI-api-key",
    baseURL: "https://api.z.ai/api/paas/v4/"
});

async function main() {
    const completion = await client.chat.completions.create({
        model: "glm-5.2",
        messages: [
            { role: "system", content: "You are a helpful AI assistant." },
            { role: "user", content: "Hello, please introduce yourself." }
        ]
    });

    console.log(completion.choices[0].message.content);
}

main();
```

**Install SDK****Maven**

```
<dependency>
    <groupId>com.openai</groupId>
    <artifactId>openai-java</artifactId>
    <version>2.20.1</version>
</dependency>
```

**Gradle (Groovy)**

```
implementation 'com.openai:openai-java:2.20.1'
```

**Usage Example**

```
import com.openai.client.OpenAIClient;
import com.openai.client.okhttp.OpenAIOkHttpClient;
import com.openai.models.chat.completions.ChatCompletion;
import com.openai.models.chat.completions.ChatCompletionCreateParams;

public class QuickStart {
    public static void main(String[] args) {
        // Initialize client
        OpenAIClient client = OpenAIOkHttpClient.builder()
            .apiKey("your-Z.AI-api-key")
            .baseUrl("https://api.z.ai/api/paas/v4/")
            .build();

        // Create chat completion request
        ChatCompletionCreateParams params = ChatCompletionCreateParams.builder()
            .addSystemMessage("You are a helpful AI assistant.")
            .addUserMessage("Hello, please introduce yourself.")
            .model("glm-5.2")
            .build();

        // Send request and get response
        ChatCompletion chatCompletion = client.chat().completions().create(params);
        Object response = chatCompletion.choices().get(0).message().content();

        System.out.println(response);
    }
}
```

###

[​

](#get-more)

Get More

[

## API Reference

Access API Reference.

](https://docs.z.ai/api-reference)

[

## Python SDK

Access Python SDK Github

](https://github.com/zai-org/z-ai-sdk-python)

[

## Java SDK

Access Java SDK Github

](https://github.com/zai-org/z-ai-sdk-java)

Was this page helpful?

YesNo

[Overview](https://docs.z.ai/guides/overview/overview)

Ctrl+I