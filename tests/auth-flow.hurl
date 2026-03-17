# SLAP Authentication Flow Test
# Tests: signup → signin → profile check
# Run: hurl --test --variable test_user_id=testuser_$(date +%s) tests/auth-flow.hurl

# Test 1: Signup - Create a new user account
POST {{base_url}}/signup
[Options]
variable: test_password=TestPassword123!
variable: test_username=Test User
[FormParams]
id: {{test_user_id}}
password: {{test_password}}
username: {{test_username}}

HTTP 303
[Captures]
signup_cookie: cookie "user_data"
[Asserts]
header "Location" == "/user/{{test_user_id}}"
cookie "user_data" exists

# Test 2: Access user profile with session cookie
GET {{base_url}}/user/{{test_user_id}}
Cookie: user_data={{signup_cookie}}

HTTP 200
[Asserts]
body contains "{{test_username}}"
body contains "{{test_user_id}}"

# Test 3: Logout
GET {{base_url}}/logout

HTTP 303
[Asserts]
header "Location" == "/"

# Test 4: Verify user is logged out (no access to profile)
GET {{base_url}}/user/{{test_user_id}}

HTTP 401
[Asserts]
body contains "Authentication required"

# Test 5: Signin with existing user
POST {{base_url}}/signin
[FormParams]
id: {{test_user_id}}
password: {{test_password}}

HTTP 303
[Captures]
signin_cookie: cookie "user_data"
[Asserts]
header "Location" == "/user/{{test_user_id}}"
cookie "user_data" exists

# Test 6: Access profile after signin
GET {{base_url}}/user/{{test_user_id}}
Cookie: user_data={{signin_cookie}}

HTTP 200
[Asserts]
body contains "{{test_username}}"
body contains "{{test_user_id}}"
body contains "Student"

# Test 7: Signup teacher — verify profile accessible
POST {{base_url}}/signup
[FormParams]
id: teacher-guid-1
password: {{test_password}}
username: Teacher User

HTTP 303
[Captures]
teacher_cookie: cookie "user_data"
[Asserts]
header "Location" == "/user/teacher-guid-1"

GET {{base_url}}/user/teacher-guid-1
Cookie: user_data={{teacher_cookie}}

HTTP 200
[Asserts]
body contains "Teacher User"

# Test 8: Test invalid signin - wrong password
POST {{base_url}}/signin
[FormParams]
id: {{test_user_id}}
password: WrongPassword123!

HTTP 200
[Asserts]
body contains "Error"
body contains "Invalid ID or password"

# Test 9: Test invalid signin - non-existent user
POST {{base_url}}/signin
[FormParams]
id: nonexistent_user
password: {{test_password}}

HTTP 200
[Asserts]
body contains "Error"
body contains "Invalid ID or password"

# Test 10: Test signup with duplicate user ID
POST {{base_url}}/signup
[FormParams]
id: {{test_user_id}}
password: {{test_password}}
username: Another User

HTTP 200
[Asserts]
body contains "Error"
body contains "User already exists"

# Test 11: Test signup with missing fields (no username)
POST {{base_url}}/signup
[FormParams]
id: newuser_missing
password: {{test_password}}

HTTP 200
[Asserts]
body contains "Error"
body contains "All fields are required"

# Test 12: Test signup with empty password
POST {{base_url}}/signup
[FormParams]
id: newuser123
password:
username: New User

HTTP 200
[Asserts]
body contains "Error"

# Test 13: Home page should be accessible
GET {{base_url}}/

HTTP 200
[Asserts]
body contains "Sign In"
body contains "Sign Up"
