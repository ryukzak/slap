# SLAP Authentication Flow Test
# Tests: signup → signin → profile check

# Test 1: Signup new user
POST http://localhost:8080/signup
[FormParams]
id: authtest999
password: TestPassword123!
username: Auth Test User

HTTP 303

# Test 2: Try accessing profile without session (expect 401)
GET http://localhost:8080/user/authtest999
HTTP 401

# Test 3: Signin with credentials
POST http://localhost:8080/signin
[FormParams]
id: authtest999
password: TestPassword123!

HTTP 303

# Test 4: Check home page
GET http://localhost:8080/
HTTP 200
