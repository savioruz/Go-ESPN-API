# API Error Keys Documentation

## Overview

All API error responses return machine-readable error keys. This allows frontends to:
- Implement consistent error handling across all endpoints
- Provide translated error messages based on user locale
- Display appropriate UI for specific error types
- Log and track error patterns

## Response Format

Error responses use two different formats depending on the error type:

### Validation Errors (422 Unprocessable Entity)

Validation errors return an `errors` array containing all validation failures:

```json
{
  "errors": [
    {
      "field": "title",
      "message": "validation.required"
    },
    {
      "field": "email",
      "message": "validation.email"
    }
  ]
}
```

Each error object contains:
- `field`: The name of the field that failed validation (in snake_case, e.g., "title", "email", "images[0]")
- `message`: An error key representing the validation rule that failed (e.g., "validation.required", "validation.email")

**Important:** The `message` field contains error keys (not human-readable text), allowing frontends to:
- Translate errors to any language
- Customize error messages per field
- Provide consistent validation UX

This format allows the API to return **all validation errors at once**, enabling frontends to highlight all problematic fields in a single response.

### Non-Validation Errors (4xx, 5xx)

All other errors return a single `error` field:

```json
{
  "error": "error.key.here"
}
```

The HTTP status code provides the error category (4xx for client errors, 5xx for server errors), and the error key provides specific details about what went wrong.

### Examples

**Validation Error (Single Field):**
```http
POST /api/galleries
422 Unprocessable Entity

{
  "errors": [
    {
      "field": "title",
      "message": "validation.required"
    }
  ]
}
```

**Validation Error (Multiple Fields):**
```http
POST /api/galleries
422 Unprocessable Entity

{
  "errors": [
    {
      "field": "title",
      "message": "validation.required"
    },
    {
      "field": "images",
      "message": "validation.url"
    }
  ]
}
```

**Resource Not Found:**
```http
GET /api/galleries/abc123
404 Not Found

{
  "error": "gallery.not_found"
}
```

**Authentication Error:**
```http
GET /api/galleries
401 Unauthorized

{
  "error": "auth.unauthorized"
}
```

**Server Error:**
```http
GET /api/galleries
500 Internal Server Error

{
  "error": "database.query_failed"
}
```

## Error Key Categories

Error keys follow the pattern: `category.specific_error`

- **validation**: Input validation errors (400, 422)
- **auth**: Authentication and authorization errors (401, 403)
- **resource**: Resource-related errors (404, 409)
- **database**: Database operation errors (500)
- **server**: General server errors (500, 503)
- **external**: External service errors (500, 503)
- **gallery**: Gallery module-specific errors (404, 500)
- **todo**: Todo module-specific errors (404, 500)

## Complete Error Key Reference

### Validation Errors (400, 422)

**Field-Specific Validation:**

Validation errors return an `errors` array with field-specific information. Each error contains:
- `field`: The field name in snake_case (e.g., `"title"`, `"user_email"`, `"images[0]"`)
- `message`: The validation rule that failed (e.g., `"validation.required"`, `"validation.email"`)
- `param` (optional): Additional context like min/max values

**Validation Message Keys:**

| Message Key | HTTP Status | Description | Example Response |
|-------------|-------------|-------------|------------------|
| `validation.required` | 422 | Required field is missing | `{"errors": [{"field": "title", "message": "validation.required"}]}` |
| `validation.min` | 422 | Value too short/small | `{"errors": [{"field": "title", "message": "validation.min", "param": "3"}]}` |
| `validation.max` | 422 | Value too long/large | `{"errors": [{"field": "description", "message": "validation.max", "param": "500"}]}` |
| `validation.gte` | 422 | Value must be ≥ param | `{"errors": [{"field": "age", "message": "validation.gte", "param": "18"}]}` |
| `validation.lte` | 422 | Value must be ≤ param | `{"errors": [{"field": "price", "message": "validation.lte", "param": "1000"}]}` |
| `validation.url` | 422 | URL format is invalid | `{"errors": [{"field": "images[0]", "message": "validation.url"}]}` |
| `validation.email` | 422 | Email format is invalid | `{"errors": [{"field": "user_email", "message": "validation.email"}]}` |
| `validation.oneof` | 422 | Value not in allowed list | `{"errors": [{"field": "status", "message": "validation.oneof"}]}` |
| `validation.mimetypes` | 422 | File type not allowed | `{"errors": [{"field": "image", "message": "validation.mimetypes"}]}` |
| `validation.maxfilesize` | 422 | File size exceeds limit | `{"errors": [{"field": "image", "message": "validation.maxfilesize"}]}` |
| `validation.empty` | 422 | Field must be empty | `{"errors": [{"field": "optional_field", "message": "validation.empty"}]}` |

**Multiple Validation Errors:**

When multiple fields fail validation, all errors are returned in the same response:

```json
{
  "errors": [
    {
      "field": "title",
      "message": "validation.required"
    },
    {
      "field": "user_email",
      "message": "validation.email"
    },
    {
      "field": "images[0]",
      "message": "validation.url"
    }
  ]
}
```

**General Validation Keys:**

These are returned for non-field-specific validation errors:

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `validation.failed` | 422 | General validation failure | Fallback for unmapped validation errors |
| `validation.invalid_page` | 400 | Invalid page parameter | Page number is negative or not a number |
| `validation.invalid_limit` | 400 | Invalid limit parameter | Limit exceeds maximum or is not a number |

### Authentication & Authorization Errors (401, 403)

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `auth.unauthorized` | 401 | User not authenticated | JWT token missing or invalid |
| `auth.forbidden` | 403 | User lacks permissions | User not allowed to access resource |
| `auth.invalid_credentials` | 401 | Login credentials incorrect | Wrong username or password |
| `auth.token_expired` | 401 | Authentication token expired | JWT token past expiration time |
| `auth.token_invalid` | 401 | Token signature/format invalid | JWT token tampered or malformed |
| `auth.resource_restricted` | 403 | Resource access restricted | User not owner of resource |
| `auth.insufficient_permission` | 403 | User role lacks permission | User is viewer, needs editor role |
| `auth.jwks_fetch_failed` | 401 | Failed to fetch JWKS | Auth service JWKS endpoint unreachable |
| `auth.invalid_claim` | 401 | Token claims invalid | Missing required claims in JWT |
| `auth.header_missing` | 401 | Authorization header missing | No Bearer token in request |
| `auth.invalid_header` | 401 | Authorization header invalid | Malformed Bearer token |
| `auth.token_parse_failed` | 401 | Token parse failed | Failed to parse JWT token |
| `auth.token_missing_kid` | 401 | Token missing key ID | JWT header missing kid |
| `auth.token_invalid_claims` | 401 | Token invalid claims | JWT claims malformed |

### Resource Errors (404, 409)

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `resource.not_found` | 404 | Generic resource not found | Requested ID doesn't exist |
| `resource.already_exists` | 409 | Resource with ID already exists | Duplicate key constraint |
| `resource.conflict` | 409 | Resource state conflict | Editing outdated version |
| `resource.locked` | 409 | Resource is locked | Another user editing resource |

### Database Errors (500)

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `database.query_failed` | 500 | Database query failed | Syntax error in SQL query |
| `database.connection_failed` | 500 | Cannot connect to database | Database server unreachable |
| `database.transaction_failed` | 500 | Transaction rollback | Deadlock or constraint violation |
| `database.constraint_violated` | 500 | Database constraint violated | Foreign key or unique constraint |

### Server Errors (500, 503)

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `server.internal_error` | 500 | Unexpected server error | Null pointer, panic, unhandled error |
| `server.service_unavailable` | 503 | Service temporarily unavailable | Server overloaded or restarting |
| `server.timeout` | 504 | Request processing timeout | Query took longer than timeout |
| `server.rate_limit_exceeded` | 429 | Too many requests | Client exceeded rate limit |
| `server.maintenance_mode` | 503 | Server in maintenance mode | Planned maintenance window |
| `server.shutting_down` | 503 | Server is shutting down | Graceful shutdown in progress |

### External Service Errors (500, 503)

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `external.service_error` | 500 | External service failure | Third-party API unavailable |
| `external.s3_upload_failed` | 500 | Failed to upload to S3 | S3 bucket permission denied |
| `external.s3_delete_failed` | 500 | Failed to delete from S3 | S3 object doesn't exist |
| `external.cache_operation_failed` | 500 | Cache operation failed | Redis connection lost |

### Gallery Module Errors

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `gallery.not_found` | 404 | Gallery not found | Gallery ID doesn't exist |
| `gallery.create_failed` | 500 | Failed to create gallery | Database insert error |
| `gallery.update_failed` | 500 | Failed to update gallery | Database update error |
| `gallery.delete_failed` | 500 | Failed to delete gallery | Database delete error |
| `gallery.list_failed` | 500 | Failed to list galleries | Database query error |
| `gallery.image_upload_failed` | 500 | Failed to upload image | S3 upload error |
| `gallery.image_delete_failed` | 500 | Failed to delete image | S3 delete error |
| `gallery.image_processing_failed` | 500 | Image processing failed | Thumbnail generation error |

### Todo Module Errors

| Error Key | HTTP Status | Description | Example Scenario |
|-----------|-------------|-------------|------------------|
| `todo.not_found` | 404 | Todo not found | Todo ID doesn't exist |
| `todo.create_failed` | 500 | Failed to create todo | Database insert error |
| `todo.update_failed` | 500 | Failed to update todo | Database update error |
| `todo.delete_failed` | 500 | Failed to delete todo | Database delete error |
| `todo.list_failed` | 500 | Failed to list todos | Database query error |

## Example API Responses

### Successful Response
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "data": {
    "id": "gallery-123",
    "title": "My Gallery",
    "images": [...]
  }
}
```

### Validation Error
```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/json

{
  "error": "validation.failed"
}
```

### Not Found Error
```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": "gallery.not_found"
}
```

### Internal Server Error
```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "error": "database.query_failed"
}
```

### Authentication Error
```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "error": "auth.unauthorized"
}
```

## Additional Resources

- **Error Key Definitions**: `shared/errkey/errkey.go`
- **Failure Package**: `shared/failure/failure.go`
- **Response Handler**: `transport/http/response/response.go`
- **API Tests**: See handler tests for examples of all error keys in action

## Support

For questions about error keys or to request new error keys, please:
1. Check this documentation first
2. Review existing error keys in `shared/errkey/errkey.go`
3. Open an issue or PR if you need a new error key added
