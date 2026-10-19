# distributed-x-infrastructure-serverless-hexagonal-api / Serverless Hexagonal Products API

[![License](https://img.shields.io/badge/License-MIT%20No%20Attribution-blue.svg)](LICENSE)
[![Huawei Cloud](https://img.shields.io/badge/Huawei%20Cloud-IAM%20Auth-red.svg)](https://support.huaweicloud.com/intl/en-us/)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](#)


---

## 1. Introduction

This practice delivers a serverless product management service organized around
**hexagonal architecture (ports and adapters)**: API Gateway exposes four REST operations,
four FunctionGraph functions handle them, product data lives in a GeminiDB
DynamoDB-compatible instance, and every product change is published as a domain event
to an event channel for downstream subscribers.

It targets **backend engineers, cloud-native architects, and DevOps engineers**, and can be
used as a reference architecture, a learning sample, or a production template.

### Key Capabilities

- **Zero cloud dependency in the domain layer** — product rules, event contracts, and error semantics live in the domain layer, which talks to the outside world only through the `Store` and `Bus` ports
- **Replaceable adapters** — database, event channel, and API Gateway protocol handling are each isolated in their own adapter, so swapping an implementation never touches business rules
- **Writes publish events** — write endpoints publish `ProductCreated`, `ProductUpdated`, and `ProductDeleted` domain events after the data is persisted
- **Complete engineering setup** — unit tests, integration tests, load-testing scripts, a CI pipeline, and one-command infrastructure provisioning

---

## 2. Table of Contents

- [1. Introduction](#1-introduction)
- [2. Table of Contents](#2-table-of-contents)
- [3. Architecture](#3-architecture)
- [4. Highlights](#4-highlights)
- [5. Huawei Cloud Services Used](#5-huawei-cloud-services-used)
- [6. Prerequisites](#6-prerequisites)
- [7. Quick Start](#7-quick-start)
- [8. Step-by-Step Deployment](#8-step-by-step-deployment)
- [9. Usage and Verification](#9-usage-and-verification)
- [10. Cleaning Up](#10-cleaning-up)
- [11. Detailed Description](#11-detailed-description)
- [12. Dependencies](#12-dependencies)
- [13. FAQ and Troubleshooting](#13-faq-and-troubleshooting)
- [14. Contributing](#14-contributing)
- [15. License](#15-license)
- [16. Contact and Maintainers](#16-contact-and-maintainers)

---

## 3. Architecture

> The domain and its ports sit at the center; every adapter sits outside it. Dependencies always point inward.

```
                            ┌────────────┐
                            │   Client   │
                            └──────┬─────┘
                                   │ HTTPS
                                   ▼
┌────────────────────────────────────────────────────────────────────┐
│                      API Gateway (Dedicated)                       │
│       GET /      GET /{id}      PUT /{id}      DELETE /{id}        │
└───────┬─────────────────┬─────────────────┬─────────────────┬──────┘
        │                 │                 │                 │
        ▼                 ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ get-products │  │ get-product  │  │ put-product  │  │delete-product│
│list products │  │ get one item │  │write + event │  │delete + event│
└───────┬──────┘  └───────┬──────┘  └───────┬──────┘  └───────┬──────┘
        │                 │                 │                 │
        └─────────────────┴────────┬────────┴─────────────────┘
                                   ▼
┌────────────────────────────────────────────────────────────────────┐
│                 Domain Layer (No Cloud Dependency)                 │
│                 Products Service + ProductsEvents                  │
└───────────────┬─────────────────────────────────────┬──────────────┘
                │ types.Store port                    │ types.Bus port
                ▼                                     ▼
┌──────────────────────────────┐      ┌──────────────────────────────┐
│        store adapter         │      │         bus adapter          │
│  protocol-compatible client  │      │   CloudEvents 1.0 envelope   │
└───────────────┬──────────────┘      └───────────────┬──────────────┘
                ▼                                     ▼
┌──────────────────────────────┐      ┌──────────────────────────────┐
│           GeminiDB           │      │         EventGrid EG         │
│ DynamoDB-compatible instance │      │     custom event channel     │
└──────────────────────────────┘      └───────────────┬──────────────┘
                                                      ▼
                                      ┌──────────────────────────────┐
                                      │    Downstream subscribers    │
                                      │     filter by event type     │
                                      └──────────────────────────────┘
```

**Key components**:

- **API Gateway** — matches requests against API definitions and delivers path parameters plus the request body to functions as an event
- **FunctionGraph** — four functions serve the four operations while sharing one domain layer and one set of adapters
- **GeminiDB DynamoDB-Compatible Instance** — stores product data; the table must be created first (the compatible interface does not create it automatically — see deployment Step 5)
- **EventGrid Custom Event Channel** — receives product change domain events for subscribers to filter and route by type

---

## 4. Highlights

- **Clean ports-and-adapters layering** — `types` declares the ports, `domain` implements business rules, and `store` / `bus` / `handlers` each wrap one external technology; the domain layer is unit-testable offline
- **Writes are bound to domain events** — a product change must produce an event, so publishing lives on the domain service rather than the entry point, and read-only entries never emit events
- **One codebase, several deployment units** — four functions are compiled from the same repository, each deciding only which capability to expose and which ports to attach
- **Runnable deliverables** — unit tests, integration tests, load-testing scripts, a CI pipeline, and one-command provisioning are all included; `make ci` builds and tests everything
- **Connection details and credentials stay out of the repository** — the storage endpoint, instance password and access keys are supplied at deployment time; no key material is stored in the code or configuration files of this repository

---

## 5. Huawei Cloud Services Used

| Service | Purpose | Notes |
|---|---|---|
| API Gateway (APIG) | Exposes REST operations and forwards requests to functions | Dedicated instance, `BASIC` edition by default |
| FunctionGraph | Hosts the four business functions on the Go runtime | Configured with VPC access and log collection |
| GeminiDB | Product data storage | DynamoDB-compatible instance, three-node cluster |
| EventGrid (EG) | Carries product change domain events | Custom event channel |
| Log Tank Service (LTS) | Collects function runtime logs | Seven-day retention by default |
| Object Storage Service (OBS) | Stores function deployment packages | Private bucket |
| Virtual Private Cloud (VPC) | Network plane for the instance and functions | Includes subnet and security group |
| Elastic IP (EIP) | Provides public access to API Gateway | Provisioned in traffic mode |
| Identity and Access Management (IAM) | Supplies access keys and resource permissions | Used for deployment and event channel calls |

> **Tip**: every service above is created by this repository's Terraform configuration. To remove them later, see [Section 10, Cleaning Up](#10-cleaning-up).

---

## 6. Prerequisites

| Item | Requirement | Notes |
|---|---|---|
| **Account** | A Huawei Cloud account with identity verification completed | Personal or enterprise accounts both work |
| **Permissions** | IAM **Tenant Administrator** or equivalent | Required to create VPC / GeminiDB / APIG / FunctionGraph resources |
| **Region** | `cn-north-4` recommended | Availability of the compatible instance and the dedicated API Gateway depends on the region |
| **Terraform** | `>= 1.5.0` | Infrastructure provisioning tool |
| **huaweicloud Provider** | `>= 1.70` | Retrieved automatically by `terraform init` |
| **Go** | `>= 1.21` | Cross-compiles the function executables |
| **zip** | Any version | Packages the function deployment archives |
| **Artillery** *(optional)* | `>= 2.0` | Load testing |
| **Access keys** | Obtain the Access Key ID and Secret Access Key from the Huawei Cloud **My Credentials** page, and note the project ID | Do **not** write credentials into any code or configuration file |

---

## 7. Quick Start

### One-Command Deployment

```bash
# 1. Clone this repository
git clone <repo-https-url> distributed-x-infrastructure-serverless-hexagonal-api
cd distributed-x-infrastructure-serverless-hexagonal-api

# 2. Compile and package the function deployment archives
make package

# 3. Provide access keys and deployment variables (see Section 6, Prerequisites)
export TF_VAR_access_key="<your-access-key>"
export TF_VAR_secret_key="<your-secret-key>"
export TF_VAR_project_id="<your-project-id>"
export TF_VAR_geminidb_password="<your-instance-password>"

# 4. Deploy
cd infra
terraform init
terraform plan
terraform apply -auto-approve
```

> **Deployment time**: roughly 8-15 minutes, since the compatible instance and the dedicated API Gateway take a while to provision.
> **Full steps with expected output**: see [Section 8, Step-by-Step Deployment](#8-step-by-step-deployment).

---

## 8. Step-by-Step Deployment

### Step 1: Compile and Package the Function Deployment Archives

```bash
cd distributed-x-infrastructure-serverless-hexagonal-api
make package
```

**Expected output**: each of the four directories under `functions/` gains a `handler`
(Linux executable) and a `function.zip`.

```bash
ls functions/*/function.zip
```

```
functions/delete-product/function.zip
functions/get-product/function.zip
functions/get-products/function.zip
functions/put-product/function.zip
```

> The compiled artifact name must match the function entry point; this repository uses `handler` throughout.
> The archives must exist before the infrastructure is applied.

### Step 2: Initialize the Working Directory

```bash
cd infra
terraform init
```

**Expected output**: `Terraform has been successfully initialized!`

### Step 3: Preview the Changes

```bash
terraform plan
```

**Expected output**: `Plan: N to add, 0 to change, 0 to destroy.`

### Step 4: Apply

```bash
terraform apply -auto-approve
```

**Expected output**: `Apply complete! Resources: N added, 0 changed, 0 destroyed.`

### Step 5: Create the Product Table

The protocol-compatible instance does not create tables on first write, so the product table is created once after the instance is ready. The primary key is a string attribute named `id`, and the request must carry provisioned read/write capacity; the on-demand capacity mode is not accepted.

The request has to originate inside the virtual private cloud of the instance. If your shell is outside that network, run it from a host in the same virtual private cloud, or temporarily from a function. With a protocol-compatible client:

```bash
aws dynamodb create-table \
  --table-name <table_name> \
  --key-schema AttributeName=id,KeyType=HASH \
  --attribute-definitions AttributeName=id,AttributeType=S \
  --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1 \
  --endpoint-url http://<geminidb_endpoint>:<geminidb_data_port>
```

**Expected output**: a `TableDescription` whose `TableStatus` is `CREATING` and turns `ACTIVE` shortly after.
The table name and endpoint come from `terraform output table_name` and `terraform output geminidb_endpoint`.

### Step 6: Record the Key Outputs

```bash
terraform output
```

**Expected output**: values such as `api_url`, `event_channel_id`, and `geminidb_endpoint`. Keep them for the verification steps below.

---

## 9. Usage and Verification

### Verification 1: Confirm the Infrastructure Is Ready

```bash
cd infra
terraform output api_url
terraform output event_channel_id
```

**Expected output**:

```
https://<apig-ingress-address>
<event-channel-id>
```

### Verification 2: Create a Product

```bash
API_URL=$(cd infra && terraform output -raw api_url)

curl -i -X PUT "$API_URL/iXR" \
  -H "Content-Type: application/json" \
  -d '{"id":"iXR","name":"iPhone XML","price":0.123}'
```

**Expected output**:

```
HTTP/2 201
content-type: application/json

{"id":"iXR","name":"iPhone XML","price":0.123}
```

### Verification 3: Read Products

```bash
# Read a single product
curl -s "$API_URL/iXR"
```

**Expected output**:

```
{"id":"iXR","name":"iPhone XML","price":0.123}
```

```bash
# Read the product list
curl -s "$API_URL"
```

**Expected output**:

```
{"products":[{"id":"iXR","name":"iPhone XML","price":0.123}]}
```

### Verification 4: Delete a Product and Confirm Idempotency

```bash
# Delete the product
curl -i -X DELETE "$API_URL/iXR"

# Read it again; a 404 is expected
curl -i "$API_URL/iXR"
```

**Expected output**:

```
HTTP/2 200
content-type: application/json

{"message":"product not found"}
```

### Verification 5: Request Validation

```bash
# The path identifier differs from the body identifier
curl -i -X PUT "$API_URL/not-the-same-id" \
  -H "Content-Type: application/json" \
  -d '{"id":"iXR","name":"iPhone XML","price":0.123}'
```

**Expected output**:

```
HTTP/2 400

{"message":"product ID in path does not match product ID in body"}
```

### Verification 6: End-to-End Business Flow (Recommended)

```bash
API_URL=$(cd infra && terraform output -raw api_url) \
  go test -v -tags=integration ./...
```

**Expected result**: every case passes, covering the full chain of create, read, list, delete, and a final read returning 404.

### Verification 7: Confirm Domain Events Were Published

After creating or deleting a product, open the corresponding event channel in the EventGrid
console and inspect the event traces. Records with `type` set to `ProductCreated`,
`ProductUpdated`, or `ProductDeleted` should be present.

```bash
cd infra
terraform output event_channel_name
```

> **Verified in practice**: every command and example in this section was actually executed before delivery,
> and each runs with output matching the description.
> Please walk through this section yourself and confirm the business functions work before treating the delivery as complete.

---

## 10. Cleaning Up

```bash
# Enter the deployment directory
cd distributed-x-infrastructure-serverless-hexagonal-api/infra

# Release all resources
terraform destroy -auto-approve

# Remove local build artifacts
cd .. && make clean
```

**Expected output**: `Destroy complete! Resources: N destroyed.`

> **Avoid leaving resources behind**:
> - `terraform destroy` releases only the resources tracked in Terraform state;
> - any object storage objects or Elastic IP addresses created manually must be released separately;
> - shared resources (such as a VPC or NAT gateway used by other workloads in the account) are not affected.

---

## 11. Detailed Description

### Workflow

Taking `PUT /{id}` as an example, a request travels as follows:

1. API Gateway matches the API definition and delivers the `id` path parameter together with the request body as an event;
2. The inbound adapter deserializes the event and builds an execution context with a deadline derived from the remaining runtime budget;
3. The entry adapter performs protocol-level validation (missing path parameter, empty body, malformed Base64);
4. The domain service parses and validates the business object, reads the current value to distinguish creation from update, and writes to storage;
5. On success, the corresponding domain event is published through the `Bus` port;
6. The result is rendered into the API Gateway response structure.

### Endpoints

| Method | Path | Function | Success status | Description |
|---|---|---|---|---|
| `GET` | `/` | `get-products` | `200` | Returns one page of products, up to 20 per page |
| `GET` | `/{id}` | `get-product` | `200` | Returns a single product, or `404` when absent |
| `PUT` | `/{id}` | `put-product` | `201` | Writes a product and publishes `ProductCreated` / `ProductUpdated` |
| `DELETE` | `/{id}` | `delete-product` | `200` | Deletes a product and publishes `ProductDeleted` |

### Request and Response Examples

```bash
# Write
curl -X PUT "$API_URL/iXR" \
  -H "Content-Type: application/json" \
  -d '{"id":"iXR","name":"iPhone XML","price":0.123}'
```

```json
{"id":"iXR","name":"iPhone XML","price":0.123}
```

```bash
# Paged query: pass the next cursor returned by the previous page
curl -s "$API_URL?next=<next-cursor>"
```

```json
{
  "products": [{"id":"iXR","name":"iPhone XML","price":0.123}],
  "next": "next-cursor"
}
```

### Domain Event Contract

| Event type | When it fires | Payload |
|---|---|---|
| `ProductCreated` | The key did not exist before the write | The newly created product |
| `ProductUpdated` | The key already existed before the write | The updated product |
| `ProductDeleted` | The key existed before the delete | The deleted product |

Events follow the CloudEvents 1.0 structure: `source` is always `products-api`, and `subject` is the product key.

### Configuration

For the full configuration index, see [`docs/deployment-guide.md`](docs/deployment-guide.md), Section 5.
For architecture design and key decisions, see [`docs/architecture.md`](docs/architecture.md).

---

## 12. Dependencies

### Main Dependencies

| Name | Version | Purpose |
|---|---|---|
| Terraform | `>= 1.5.0` | Infrastructure provisioning |
| huaweicloud Provider | `>= 1.70` | Huawei Cloud resource management |
| Go | `>= 1.21` | Compiles the function code |
| FunctionGraph Go Runtime | Bundled with the repository | Function entry registration and event context |
| EventGrid Open API Client | `v0.1.215` | Domain event publishing |
| Compatible Instance Wire-Protocol Client | `v1.30.4` | Data access against the compatible instance |
| gomock | `v1.6.0` | Test doubles for unit tests |

> The FunctionGraph Go runtime ships with the repository under `go-runtime/`.
> The `replace` directive in `go.mod` points at that local path, so no network access is needed at build time.

---

## 13. FAQ and Troubleshooting

### Q1: Deployment reports insufficient permissions. What should I check?

**A**: Confirm the account holds IAM **Tenant Administrator**, or at least the administrator roles of the
relevant services such as `VPC Administrator`, `APIG Administrator`, and `FunctionGraph Administrator`.

### Q2: The function reports an authentication failure when writing. Why?

**A**: The data plane of the compatible instance accepts only the **instance's own database account**;
IAM credentials and delegated temporary credentials are not accepted. Verify that the instance password
in the function runtime environment matches the password set when the instance was created.

### Q3: The function times out connecting to storage. Why?

**A**: The data plane port depends on the deployment form — **a cluster uses the high-availability port 80,
while a single-node instance uses 8000**. Using the wrong port shows up as a connection timeout rather than
an authentication failure. Probe the open port and then adjust the `geminidb_data_port` variable before reapplying.

### Q4: Instance creation fails with `Invalid database version`.

**A**: The compatible interface exposes no version number, so `datastore.version` must be an empty string;
a concrete version is rejected by the service.

### Troubleshooting Checklist

| Symptom | Likely cause | How to investigate |
|---|---|---|
| `terraform apply` reports a missing deployment archive | `make package` was not run | Run `make package` in the repository root and retry |
| The API returns `missing 'id' parameter in path` | The last path segment is empty | Confirm the request path looks like `/iXR` with a non-empty last segment |
| The API returns `empty request body` | The body is empty or `Content-Type` is missing | Provide a JSON body and the `Content-Type` header |
| A write returns `500` and logs show a connection failure | The data plane port or network plane is misconfigured | See Q3; verify the port and the VPC configuration |
| Events never reach subscribers | The event channel or credentials are invalid | Search the LTS logs for the `failed to publish` keyword |

---

## 14. Contributing

Issues and pull requests are welcome. Please follow this flow:

1. **Issue**: describe the problem or suggestion, including reproduction commands and expected versus actual output
2. **PR**: branch off `main`; run `make ci` and `terraform fmt -check` before submitting
3. **Code review**: merged after approval by **[@\<github-username\>](https://github.com/\<github-username\>)**

---

## 15. License

This practice is released under the **MIT No Attribution** license.
See the [`LICENSE`](LICENSE) file in the repository root.

```
MIT No Attribution

Copyright Huawei Cloud. All Rights Reserved.

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

## 16. Contact and Maintainers

| Item | Value |
|---|---|
| **Created by** |
| **Maintained by** | 
| **Contact** | GitHub Issues 

---
