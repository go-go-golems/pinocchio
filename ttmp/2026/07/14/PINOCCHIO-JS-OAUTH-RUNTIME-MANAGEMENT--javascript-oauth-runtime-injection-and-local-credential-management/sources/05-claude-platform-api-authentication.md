The Claude API supports two ways to authenticate requests:

| Method | Credential | Best for |
| --- | --- | --- |
| [API key](#api-keys) | Static `sk-ant-api...` secret in the `x-api-key` header | Local 
development, prototyping, scripts, and single-tenant servers where you control secret storage |
| [Workload Identity Federation](#workload-identity-federation) | Short-lived bearer token 
exchanged from your identity provider's identity token | Production workloads on cloud platforms 
(AWS, Google Cloud, Azure), CI/CD pipelines, and Kubernetes, where you want to eliminate static 
secrets |

Both methods grant the same access to Claude API endpoints. Choose API keys to get started quickly, 
and move to Workload Identity Federation when your workload already has a platform-issued identity 
you can federate.

## API keys

API keys are static secrets that you generate in the Claude Console and pass on every request.

- **Create a key:** Go to [Settings → API keys](https://platform.claude.com/settings/keys) in the 
Claude Console. You choose an [expiration](#key-expiration) as part of creation. Use 
[workspaces](https://platform.claude.com/settings/workspaces) to scope keys by project or 
environment.
- **Send the key:** Set the `x-api-key` header on direct HTTP requests, or set the 
`ANTHROPIC_API_KEY` environment variable and the [client 
SDKs](https://platform.claude.com/docs/en/cli-sdks-libraries/overview) pick it up automatically.

```
POST /v1/messages
x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01
content-type: application/json
```

Store API keys in a secrets manager, rotate them periodically, and revoke any key you suspect has 
leaked. You can also set an [expiration](#key-expiration) when you create a key to limit how long a 
leaked credential stays usable.

```
client = Anthropic(api_key="my-anthropic-api-key")
# or, with ANTHROPIC_API_KEY set in the environment:
client = Anthropic()
```

### Key expiration

When you create an API key from the [API keys page](https://platform.claude.com/settings/keys) in 
the Claude Console, you choose an expiration: a preset (3 hours, 1 day, 7 days, or 30 days), a 
custom duration, or **Never** for keys you store in a secrets manager and rotate yourself. If your 
organization has a maximum expiration policy, the Console limits presets and custom durations to 
the policy maximum, and **Never** is unavailable. Existing keys keep their current behavior; 
expiration is set at creation time and cannot be changed afterward. The same expiration choice 
applies when you [create an Admin API 
key](https://platform.claude.com/docs/en/manage-claude/admin-api-keys) in the Claude Console.

Anthropic emails the key's creator as the expiration approaches: 7 days before expiration for keys 
created with a lifetime of at least 14 days, and 1 day before for keys with a lifetime of at least 
7 days. Keys with shorter lifetimes expire without a warning email.

After a key expires, requests made with it return a `401 authentication_error`. Create a new key to 
restore access; expired keys cannot be reactivated.

The Console API keys table shows each key's expiration, and the Admin API reports each key's 
`expires_at` timestamp on the [List API 
Keys](https://platform.claude.com/docs/en/api/admin/api_keys/list) and [Get API 
Key](https://platform.claude.com/docs/en/api/admin/api_keys/retrieve) endpoints, so you can audit 
and rotate keys before they expire. The field is `null` for keys without an expiration.

Expiration limits the lifetime of a leaked credential, but it is not a substitute for secret 
hygiene. Regardless of expiration, store keys in a secrets manager and revoke any key you suspect 
has leaked.

Workload Identity Federation (WIF) lets a workload authenticate with a short-lived identity token 
issued by an identity provider (IdP) you already trust, such as AWS IAM, Google Cloud, or any 
standards-compliant OIDC issuer (such as GitHub Actions, Kubernetes service accounts, SPIFFE, 
Microsoft Entra ID, or Okta). The workload exchanges its IdP-issued JWT at `POST /v1/oauth/token` 
for a short-lived Claude API access token, and the SDK refreshes that token automatically before it 
expires. There is no `sk-ant-api...` string to mint, distribute, or rotate.

Federation removes long-lived Claude API keys from your environment, which shrinks the blast radius 
of a leaked credential and lets you manage access with the same IdP controls you already use for 
cloud resources. It does not, on its own, guarantee end-to-end security: the trust chain is only as 
strong as your identity provider's configuration, and a long-lived secret one hop upstream (for 
example, a static cloud credential that can mint IdP tokens) can still undermine it. Pair 
federation with your provider's controls, such as IP allowlists, MFA, and audit logging.

To configure federation, you create three resources in the Claude Console (a service account, a 
federation issuer, and a federation rule) and then point your SDK at the rule. See [Workload 
Identity 
Federation](https://platform.claude.com/docs/en/manage-claude/workload-identity-federation) for the 
full setup walkthrough.[Set up Workload Identity 
Federation](https://platform.claude.com/docs/en/manage-claude/workload-identity-federation)

[

Configure issuers, rules, and service accounts, then exchange tokens

](https://platform.claude.com/docs/en/manage-claude/workload-identity-federation)[

Identity provider guides

Step-by-step guides for AWS, Google Cloud, Azure, GitHub Actions, Kubernetes, SPIFFE, and Okta

](https://platform.claude.com/docs/en/manage-claude/workload-identity-federation#identity-providers)
[

WIF reference

Environment variables, validation rules, profile configuration, and error reference

](https://platform.claude.com/docs/en/manage-claude/wif-reference)[

Client SDKs

Python, TypeScript, C#, Go, Java, PHP, Ruby, and the CLI

](https://platform.claude.com/docs/en/cli-sdks-libraries/overview)

Was this page helpful?
source_url: https://platform.claude.com/docs/en/manage-claude/authentication
