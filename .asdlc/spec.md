# Overview

This project adds a new PATCH endpoint to an existing todo management system, enabling clients to mark an individual todo item as completed. The capability is intended for end users of the todo application who need a quick, unambiguous way to update the status of a task without modifying its other fields.

The endpoint will expose a focused, partial-update operation that changes only the completion status of a specified todo. It is aimed at frontend applications, integrations, and automated workflows that need reliable, predictable behavior when transitioning a todo from an incomplete to a completed state.

By scoping the change to a single well-defined action, the system keeps the API surface small, reduces the risk of unintended side effects from broader updates, and gives clients a clear contract for completion semantics.

# Capabilities

## Endpoint Behavior
- Provide a PATCH endpoint that marks a single todo as completed, addressed by the todo's unique identifier.
- Accept requests that identify the target todo via a path parameter.
- Update only the completion status field; leave all other fields of the todo unchanged.
- Return the updated todo representation in the response body on success.
- Be idempotent: marking an already-completed todo as completed must succeed without creating duplicate state changes.
- Record or update a completion timestamp when a todo transitions to completed.

## Input Validation
- Validate that the provided todo identifier is well-formed before processing.
- Reject requests referencing a non-existent todo with a clear not-found response.
- Ignore or reject unexpected fields in the request body according to a consistent policy.
- Require no additional body fields to perform the completion action.

## Response and Status Codes
- Return a success status code when a todo is successfully marked as completed.
- Return a not-found status code when the specified todo does not exist.
- Return a client-error status code for malformed identifiers or invalid requests.
- Return a server-error status code for unexpected failures, without leaking internal details.
- Include a consistent, machine-readable error body for all error responses.

## Data Integrity
- Ensure the completion update is persisted durably before returning a success response.
- Prevent partial updates: either the todo is fully marked completed (with timestamp) or no change is made.
- Handle concurrent completion requests on the same todo safely without corrupting state.

## Authentication and Authorization
- Require the request to be authenticated using the system's existing authentication mechanism.
- Authorize only users who own or have permission to modify the specified todo.
- Reject unauthenticated requests with an unauthorized response.
- Reject authenticated but unauthorized requests with a forbidden response.

## Observability
- Log each completion request with the todo identifier, requesting user, outcome, and timestamp.
- Emit metrics for request count, success rate, error rate, and latency of the endpoint.
- Ensure logs and metrics do not contain sensitive user data.

## Performance and Reliability
- Respond to valid completion requests within a predictable low-latency threshold under normal load.
- Sustain the endpoint's expected request volume without degradation of other system endpoints.
- Gracefully handle transient backend failures with appropriate retry-safe error responses.

## Documentation
- Document the endpoint's path, method, parameters, request schema, and response schema.
- Provide example requests and responses for success and common error cases.
- Document authentication requirements and authorization rules for the endpoint.
