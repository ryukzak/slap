# Cross-Origin Protection Test
# Tests: a browser POST from another site or a sibling subdomain is rejected
#        before it reaches a handler; same-origin POSTs, non-browser POSTs
#        and cross-site GETs (e.g. a lesson link opened from a messenger)
#        still work
#
# Prerequisites: server running.
# Run: hurl --test tests/csrf-protection.hurl

# Step 1: POST marked cross-site by the browser is rejected
POST {{base_url}}/reset-request
Sec-Fetch-Site: cross-site
[FormParams]
id: abc

HTTP 403

# Step 2: POST from a sibling subdomain (same site, other origin) is rejected
POST {{base_url}}/reset-request
Sec-Fetch-Site: same-site
[FormParams]
id: abc

HTTP 403

# Step 3: Without Sec-Fetch-Site, a POST from a foreign Origin is rejected
POST {{base_url}}/reset-request
Origin: https://other.example
[FormParams]
id: abc

HTTP 403

# Step 4: Same-origin POST reaches the handler
POST {{base_url}}/reset-request
Sec-Fetch-Site: same-origin
[FormParams]
id: abc

HTTP 200
[Asserts]
body contains "ID must contain digits only"

# Step 5: Non-browser POST (no Sec-Fetch-Site, no Origin) reaches the handler
POST {{base_url}}/reset-request
[FormParams]
id: abc

HTTP 200
[Asserts]
body contains "ID must contain digits only"

# Step 6: Cross-site GET still works
GET {{base_url}}/
Sec-Fetch-Site: cross-site

HTTP 200
