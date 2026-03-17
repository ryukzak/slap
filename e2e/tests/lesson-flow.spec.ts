import { test, expect, BrowserContext, Page } from '@playwright/test';
import { signup, uniqueId, TEACHER_ID, TEACHER_PASSWORD, TEACHER_USERNAME } from './helpers';

async function getTeacherPage(context: BrowserContext): Promise<Page> {
  const teacherPage = await context.browser()!.newContext().then(c => c.newPage());
  await teacherPage.goto('/');

  // Try signup first; if user already exists, fall back to signin
  await teacherPage.fill('#signup-id', TEACHER_ID);
  await teacherPage.fill('#signup-username', TEACHER_USERNAME);
  await teacherPage.fill('#signup-password', TEACHER_PASSWORD);
  await teacherPage.click('form[hx-post="/signup"] button[type="submit"]');

  // Wait for navigation or error
  await teacherPage.waitForFunction(
    () => window.location.pathname.includes('/user/') ||
          (document.body.textContent || '').includes('User already exists')
  );

  // If "User already exists" appeared, the auth page is still showing — use signin
  const bodyText = await teacherPage.locator('body').textContent();
  if (bodyText?.includes('User already exists')) {
    await teacherPage.fill('#signin-id', TEACHER_ID);
    await teacherPage.fill('#signin-password', TEACHER_PASSWORD);
    await teacherPage.click('form[hx-post="/signin"] button[type="submit"]');
    await teacherPage.waitForURL(/\/user\//);
  }

  return teacherPage;
}

test('teacher creates lesson and it appears in lesson list', async ({ page }) => {
  const teacherPage = await getTeacherPage(page.context());

  await teacherPage.fill('textarea[name="description"]', 'E2E Test Lesson');
  await teacherPage.fill('input[name="datetime"]', '2099-12-31T14:00');
  const lessonResponse = teacherPage.waitForResponse(r => r.url().includes('/api/lessons'));
  await teacherPage.click('button[type="submit"][class*="bg-blue"]');
  await lessonResponse;

  await teacherPage.reload();
  await expect(teacherPage.locator('body')).toContainText('E2E Test Lesson');

  await teacherPage.context().close();
});

test('student sees lesson list on dashboard', async ({ page }) => {
  const id = uniqueId('student');
  await signup(page, id, 'Lesson Viewer', 'StudentPass123!');

  await page.goto(`/user/${id}`);
  await expect(page.locator('body')).toContainText('Lessons');
});

test('student can register task for lesson and sees Registered badge', async ({ page }) => {
  // Ensure teacher and a lesson exist
  const teacherPage = await getTeacherPage(page.context());
  await teacherPage.fill('textarea[name="description"]', 'Register Test Lesson');
  await teacherPage.fill('input[name="datetime"]', '2099-12-31T14:00');
  await teacherPage.click('button[type="submit"][class*="bg-blue"]');
  await teacherPage.context().close();

  const studentId = uniqueId('student');
  await signup(page, studentId, 'Register Student', 'StudentPass123!');

  // Submit a task record first
  await page.goto(`/user/${studentId}/task/task1`);
  await page.fill('textarea[name="content"]', 'Ready for the lesson');
  await page.click('button[type="submit"]');
  await page.waitForURL(`/user/${studentId}/task/task1`);

  // Wait for HTMX to load the lesson list
  await page.waitForSelector('button:has-text("[register]")');

  // Register for a lesson
  await page.click('button:has-text("[register]")');

  // Should redirect back to task page with Queued badge
  await page.waitForURL(`/user/${studentId}/task/task1`);
  await expect(page.locator('body')).toContainText('Queued');
});

test('new submission revokes lesson registration', async ({ page }) => {
  // Ensure teacher and a lesson exist
  const teacherPage = await getTeacherPage(page.context());
  await teacherPage.fill('textarea[name="description"]', 'Revoke Test Lesson');
  await teacherPage.fill('input[name="datetime"]', '2099-12-31T14:00');
  await teacherPage.click('button[type="submit"][class*="bg-blue"]');
  await teacherPage.context().close();

  const studentId = uniqueId('student');
  await signup(page, studentId, 'Revoke Student', 'StudentPass123!');

  // Submit and register
  await page.goto(`/user/${studentId}/task/task1`);
  await page.fill('textarea[name="content"]', 'First submission');
  await page.click('button[type="submit"]');
  await page.waitForURL(`/user/${studentId}/task/task1`);

  await page.waitForSelector('button:has-text("[register]")');
  await page.click('button:has-text("[register]")');
  await page.waitForURL(`/user/${studentId}/task/task1`);
  await expect(page.locator('body')).toContainText('Queued');

  // Submit a new record — should revoke registration
  await page.fill('textarea[name="content"]', 'Updated submission');
  await page.click('button[type="submit"]');
  await page.waitForURL(`/user/${studentId}/task/task1`);

  await expect(page.locator('body')).toContainText('Dropped');
  // Latest record has no registration
  await expect(page.locator('body')).not.toContainText('Queued for');
});

test('access control: student cannot view another student profile', async ({ page }) => {
  const studentAId = uniqueId('student_a');
  const studentBId = uniqueId('student_b');

  await signup(page, studentBId, 'Student B', 'StudentPass123!');

  // Now sign in as student A
  const contextA = await page.context().browser()!.newContext();
  const pageA = await contextA.newPage();
  await signup(pageA, studentAId, 'Student A', 'StudentPass123!');

  // Student A tries to view student B's profile
  const response = await pageA.request.get(`/user/${studentBId}`, {
    headers: { Cookie: await contextA.cookies().then(
      cs => cs.map(c => `${c.name}=${c.value}`).join('; ')
    ) }
  });
  expect(response.status()).toBe(403);

  await contextA.close();
});
