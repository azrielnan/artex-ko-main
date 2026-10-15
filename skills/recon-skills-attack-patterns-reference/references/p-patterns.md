# Extended Web Pattern Notes

Use these notes to route an observation to focused validation. Severity depends
on the data, identity, control boundary, and impact demonstrated on the current
target.

## WordPress and CORS

| ID | Pattern | Evidence required |
|---|---|---|
| P-01 | WordPress REST user metadata | Valid JSON user objects, not a catch-all response |
| P-02 | Reflected credentialed CORS | Untrusted origin, `ACAC: true`, and browser-readable protected data |
| P-03 | Null-origin trust | Sandboxed origin reads protected data with an approved session |
| P-04 | Wildcard CORS | Non-public response is readable without credentials |
| P-05 | Credentialed preflight | Preflight and actual request both succeed in a browser |
| P-06 | CORS on a protected route | Approved identity receives protected data cross-origin |
| P-07 | XML-RPC multicall | Protocol-valid response for multiple bounded method calls |
| P-08 | XML-RPC SSRF | Callback received by an operator-controlled service |
| P-09 | Metadata reachability through SSRF | Controlled evidence beyond a generic `faultCode` |
| P-10 | Registration to privileged upload | Synthetic account receives an upload-capable role and approved test file executes |
| P-11 | Plugin namespace | Response content identifies the plugin or capability |
| P-12 | Author sitemap metadata | Valid author records containing non-public identity data |
| P-13 | Weaker staging control | Same test behaves differently across comparable environments |
| P-14 | Unconfigured WordPress installer | Installer content is valid and the site is actually unconfigured |
| P-15 | Exposed error log | Response contains real application errors or sensitive runtime data |
| P-16 | PHPInfo exposure | PHP configuration content is present; impact depends on exposed settings |
| P-17 | Source or backup leak | File signature and meaningful source, configuration, or database content |
| P-18 | JavaScript credential candidate | Value is live, privileged, and not an intentionally public identifier |
| P-19 | Public database service | Service is reachable and access control is tested only as authorized |
| P-20 | Exposed internal application port | Service identity and unauthorized function are demonstrated |
| P-21 | WooCommerce API presence | API route is present; access still requires separate authorization testing |
| P-22 | Plugin stack trace | Response exposes non-public paths, code, queries, or configuration |
| P-23 | Shared-hosting relationship | Ownership and trust relationship are confirmed, not inferred from IP alone |
| P-24 | IAM-role path through SSRF | Controlled proof establishes a reachable role path and resulting capability |
| P-25 | CORS across REST namespaces | Each relevant endpoint passes browser and data-sensitivity validation |

## Negative Controls

- Compare suspicious paths with a random nonexistent path to detect catch-all
  routing.
- Repeat authenticated behavior without a session and with a second approved
  identity.
- Compare the exact component version and configuration with the vulnerability
  prerequisites.
- For write tests, create and clean up a synthetic object instead of modifying
  an existing record.
- Stop when the next step would exceed scope, increase availability risk, or
  collect more data than required to prove the behavior.
