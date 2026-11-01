# Terraform provider for Calendoro

Resources: `calendoro_event_type`, `calendoro_webhook`, `calendoro_schedule`, `calendoro_managed_attribute`, `calendoro_managed_role`, `calendoro_managed_template`. All support `terraform import` by id. See `examples/main.tf`.

The table in `internal/crud/resources_gen.go` is generated from OpenAPI by `scripts/generate-sdks.ts`. Scalars are typed attributes; objects and arrays are JSON text attributes named `*_json`, written with `jsonencode()`.

Known limits:

- A null or removed optional attribute is not sent, so it does not clear the server's value.
- A webhook `secret` is only shown at creation, so it is empty after an import.
- `custom_headers_json` is write-only on the API, so changes made elsewhere are not detected.
- Resources with no get-by-id (all but event types) are read by scanning the list.

Tests in `internal/crud` run against a local stub with `go test ./...`. Nothing here has been exercised by a real `terraform apply`.

Publishing needs a repository named `terraform-provider-calendoro` and a registered GPG key: the Terraform Registry reads from that repository, not a subfolder of this one.
